package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

/*
search for user (blocking i.e. will wait for Exists() to resolve).
if not already exist then fire off goroutines to create new user and session
in no specific order (concurrently): leverage context switching as mongo api calls
will place its goroutine into waiting state.
https://stackoverflow.com/questions/43021058/golang-read-request-body-multiple-times
*/
func Register(collections ...*mongo.Collection) http.Handler {
	// collections[0] is user collections[1] is session
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// set http headers for when need to send response back
		w.Header().Set("Content-Type", "application/json")

		// pull out json from request body into a byte slice then into a nice map for late use
		var userInputMap map[string]string
		// fmt.Printf("body value: %v, body type: %T", (*r).Body, (*r).Body)
		userInputBytes, err := io.ReadAll(r.Body)
		if err != nil {
			json.NewEncoder(w).Encode("error: no body could be read")
		}
		err = json.Unmarshal(userInputBytes, &userInputMap)
		if err != nil {
			fmt.Println("couldn't unmarshal the byte slice representation of JSON into a map")
		}
		// construct bson document that has user field to search for existence
		var userAsBSOND bson.D
		userAsBSOND = append(userAsBSOND, bson.E{Key: "user", Value: userInputMap["user"]})

		// this has to block, because whether or not user exists determines course of action
		found, err := Exists(userAsBSOND, collections[0])
		if err != nil {
			fmt.Println("could not complete search for user in sessions collection")
		}
		if found {
			json.NewEncoder(w).Encode(fmt.Sprintln("user already exists"))
			fmt.Println(found, userAsBSOND)
			return
		}

		// run the two creates using goroutines to leverage context switching while InsertOne
		// is waiting (InsertOne blocks rest of Create...()); spawn goroutines for performance
		userChan := make(chan *mongo.InsertOneResult)
		sessionChan := make(chan string)
		go CreateNewUser(userChan, userInputMap, collections[0])
		go CreateNewSession(sessionChan, userInputMap, collections[1])

		// prepare response while considering potential timeout
		// if after 2 seconds both sessionID and user result not provided, timeout
		var msg, cookieName string
		for numAppended := 0; numAppended < 2; numAppended++ {
			select {
			case sess := <-sessionChan:
				msg += fmt.Sprintf("session: %v\n", string(sess))
				cookieName = fmt.Sprintf("%v", string(sess))
			case user := <-userChan:
				msg += fmt.Sprintf("user: %v\n", *user)
			case <-time.After(2 * time.Second):
				msg = "registration timed out: invalid user info or backend issue"
				numAppended = 2 // needed to break out of for loop
			}
		}
		// ask client to set a cookie, so set Set-Cookie in header according to mdn docs
		w.Header().Set("Set-Cookie", fmt.Sprintf("session-id=%s", cookieName))
		json.NewEncoder(w).Encode(msg)
	})
}

/*
1. dispatch goroutine: search for user in user collection.
2. dispatch goroutine: search for session using field "user" instead of "session".
firing off 1. and 2. at same time to leverage context switching: each goroutine will
be in waiting state for mongo api calls FindOne() and InsertOne() to resolve.
to be considered 'logged in':
  - 1. exists and 2. not exists: create session document in db, Set-Cookie in response
  - 1. exists and 2. exists:
    if request cookie different to generated session cookie by mongo, Set-Cookie
*/
func Login(collections ...*mongo.Collection) http.Handler {
	// collections[0] is user collections[1] is session
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// set http headers for when need to send response back
		w.Header().Set("Content-Type", "application/json")

		// pull out json from request body into a byte slice then into a nice map for later use
		var userInputMap map[string]string
		// fmt.Printf("body value: %v, body type: %T", (*r).Body, (*r).Body)
		userInputBytes, err := io.ReadAll(r.Body)
		if err != nil {
			json.NewEncoder(w).Encode("error: no body could be read")
		}
		err = json.Unmarshal(userInputBytes, &userInputMap)
		if err != nil {
			fmt.Println("couldn't unmarshal the byte slice representation of JSON into a map")
		}
		var sessionAsBSOND bson.D
		sessionAsBSOND = append(sessionAsBSOND, bson.E{Key: "user", Value: userInputMap["user"]})

		/* run the two searches as specified above to leverage context switching; performance.
		e.g. FindOne is mongo api call; puts goroutine (the one calling Exists()) in waiting state.
		can context switch to another goroutine (the one calling FindSession) */
		// user search goroutine tell login goroutine whether or not user is valid in db
		grlchangru := make(chan bool)
		// session goroutine sends login goroutine the session string (for cookie use)
		grlchangrs := make(chan string)
		// search for user existence in user collection
		go func() {
			userExists, err := VerifyUserCredentials(userInputMap, collections[0])
			if err != nil {
				log.Fatal(err)
			}
			grlchangru <- userExists
		}()
		// search for session existence in session collection
		go func() {
			session, err := FindSession(sessionAsBSOND, collections[1])
			if err != nil {
				log.Fatal(err)
			}
			grlchangrs <- session // not process anything else, so block until elsewhere read out
		}()

		var msg, cookie string
		// could use waitgroups but ugly when do two wg.Done()'s if !userExists :-)
		for numReceives := 0; numReceives < 2; numReceives++ {
			select {
			case session := <-grlchangrs:
				if len(session) > 0 { // found a valid session but not sure if user valid
					cookie = session
				} else {
					/* can create a new session even if haven't verified existence of a user yet; for
					any valid session id to be sent back to client, user credentials will first be
					verified anyways. */
					chanSString := make(chan string)
					go CreateNewSession(chanSString, userInputMap, collections[1])
					cookie = <-chanSString
				}
				msg = "logged in" // now has a valid session but if user not exist msg gets overwritten
			case userExists := <-grlchangru:
				if !userExists {
					// rewrite Set-Cookie headers if somehow a valid session was set by above
					// prevent bug if there is a session document for a user but user not registered
					cookie = "oopsSomebodysUnauthenticated"
					msg = "wrong login credentials. if you forgot your username try registering again"
					// skip straight out of for loop: prevent any overwriting from other case (safety)
					numReceives = 2
				}
			}
		}
		// doesn't matter if client already has cookie set in header, overwrite
		w.Header().Set("Set-Cookie", fmt.Sprintf("session-id=%s", cookie))
		json.NewEncoder(w).Encode(msg)
	})
}
