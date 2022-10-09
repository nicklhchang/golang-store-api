package auth

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func init() {
	// need .env file in this directory
	err := godotenv.Load("./.env")
	if err != nil {
		log.Fatal(err)
	}
}

const cookieCharacters = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var randSeed *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

// inserts doc into sessions collection. doc is the current session of authed user.
// is spawned as a goroutine and creates a session. then this goroutine will spawn
// another to delete session doc once timed out.
func CreateNewSession(channel chan<- string, userInfo map[string]string,
	sCollection *mongo.Collection) {
	// in future could consider maybe reddis or just a global slice for speed
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// empty initialisation rather than nil (var sessionDocument bson.M)
	sessionDocument := make(bson.M)
	sessionDocument["user"] = userInfo["user"]
	// generate a sessionID to be set in client's Cookie header
	sessionID := make([]byte, 64)
	for idx := range sessionID {
		// Intn() safe to be used cocurrently (if other goroutines do randSeed.Intn())
		// because go run -race reveals it is a Read, not Write
		sessionID[idx] = cookieCharacters[randSeed.Intn(len(cookieCharacters))]
	}
	sessionDocument["session"] = string(sessionID)
	/* // to use upsert
	sessionDocument["_id"] = string(sessionID) */
	// puts goroutine into waiting state: opportunity for context switch
	_, err := sCollection.InsertOne(ctx, sessionDocument)
	if err != nil {
		fmt.Println("mongo error inserting new session document")
		return
	}
	// another goroutine so won't block the statement: channel <- string(sessionID)
	go DeleteSessionTimeout(sessionDocument, sCollection)
	channel <- string(sessionID)
}

func FindSession(searchSession bson.D, collection *mongo.Collection) (string, error) {
	// result will just be a map e.g. access username value by result["user"]
	var result bson.M
	// puts goroutine into waiting state: opportunity for context switch
	err := collection.FindOne(context.TODO(), searchSession).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", nil
		}
		log.Fatal(err)
		return "", err
	}
	// assumes all values for key "session" will be stored as string
	return result["session"].(string), nil
}

// deletes session doc from mongo sessions collection. is spawned as a goroutine
// by createNewSession so that it can clean up after itself.
// if deletion needs to be cancelled consider a select statement: cancellation channel
// must receive signal (<-) before <-timeoutChan.C. cancellation maybe because
// another goroutine has deleted already so no point making another trip to db
func DeleteSessionTimeout(session bson.M, sCollection *mongo.Collection) {
	timeoutChan := time.NewTimer(time.Duration(600 * time.Second))
	<-timeoutChan.C // halt execution of this goroutine here until timeout
	// doesn't matter how long it takes, session has to be deleted
	_, err := sCollection.DeleteOne(context.TODO(), session)
	if err != nil {
		fmt.Println("user session unable to be invalidated at this time")
	}
}

// for now almost same implementation as creating new session for authed user.
// just don't dispatch a deletion scheduled after 90 second
// no timeout on InsertOne either because important that user is registered in db
func CreateNewUser(channel chan<- *mongo.InsertOneResult, userInfo map[string]string,
	uCollection *mongo.Collection) {
	/*
		user documents have field "user" and "password" for now. if want "email" will need
		an index with unique:true on both fields "user" and "email"
	*/
	pwdSaltedHashed := PwdStringToHashedHex(userInfo["pwd"])
	// prepare bson.M for insertion into mongoDB
	userDocument := make(bson.M)
	userDocument["user"] = userInfo["user"]
	userDocument["pwd"] = pwdSaltedHashed
	newUser, err := uCollection.InsertOne(context.TODO(), userDocument)
	if err != nil {
		fmt.Println("mongo error inserting new user record")
	}
	channel <- newUser
}

// really piggybacking off of Exists() except need to salt-hash user supplied pwd
func VerifyUserCredentials(userInfo map[string]string,
	uCollection *mongo.Collection) (bool, error) {
	pwdSaltedHashed := PwdStringToHashedHex(userInfo["pwd"])
	// bson.D format of user for Exists()
	var user bson.D
	user = append(user, bson.E{Key: "user", Value: userInfo["user"]})
	user = append(user, bson.E{Key: "pwd", Value: pwdSaltedHashed})
	exists, err := Exists(user, uCollection)
	return exists, err
}
