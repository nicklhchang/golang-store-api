package auth

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// pass client supplied pwd field, append SALT (from .env) then hash with sha512 package
func PwdStringToHashedHex(userInputPwd string) string {
	// salt as a byte slice (to be concatenated to password)
	salt := []byte(os.Getenv("SALT"))
	// user supplied password as byte slice
	pwdBytes := []byte(userInputPwd)
	pwdBytes = append(pwdBytes, salt...)
	// hash salted password and encode into a hex string for storage in mongoDB
	hasher := sha512.New()
	hasher.Write(pwdBytes)
	hashedPwdBytes := hasher.Sum(nil)
	hashedPwdHex := hex.EncodeToString(hashedPwdBytes)
	return hashedPwdHex
}

/*
protected routes sit behind this: extracts cookie from http header and does db lookup
to verify is a valid session. return type allows specifying a collection and still being
able to wrap and return a function that implements interface http.Handler
*/
func AuthMiddleware(sCollection *mongo.Collection) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			ptrCookieSlice := r.Cookies()
			// fmt.Printf("cookies: %v\n", ptrCookieSlice)
			var sessionID string
			for _, ptrCookie := range ptrCookieSlice {
				if (*ptrCookie).Name == "session-id" {
					sessionID = (*ptrCookie).Value
				}
			}
			if len(sessionID) > 0 {
				exists, err := Exists(bson.D{{Key: "session", Value: sessionID}}, sCollection)
				if err != nil {
					log.Fatal(err) // something wrong with db
				}
				switch exists {
				case true:
					json.NewEncoder(w).Encode(fmt.Sprintf("session id: %s", sessionID))
					handler.ServeHTTP(w, r)
				case false:
					json.NewEncoder(w).Encode(fmt.Sprintf("session id: %s invalid", sessionID))
				}
			} else {
				json.NewEncoder(w).Encode("no session id, check your cookies")
			}
		})
	}
}

func Exists(searchParams bson.D, collection *mongo.Collection) (bool, error) {
	// no timeout context because NEED to find whether or not user or session exists
	var result bson.M
	err := collection.FindOne(context.TODO(), searchParams).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		log.Fatal(err)
		return false, err
	}
	// smoothly found: no ErrNoDocuments and unmarshaling by Decode went smooth
	return true, nil
}
