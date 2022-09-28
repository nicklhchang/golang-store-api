package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	auth "gorilla-mongo-api/auth"
)

var sessionCollection *mongo.Collection
var userCollection *mongo.Collection
var collections []*mongo.Collection

func init() {
	// for initialising any constants/globals rest of program can access
	err := godotenv.Load("./.env")
	if err != nil {
		log.Fatal(err)
	}

	// mongoClient represents connection to mongo instance
	mongoClient, err := mongo.Connect(context.TODO(),
		options.Client().ApplyURI(os.Getenv("MONGO_URI")))
	if err != nil {
		log.Fatal(err)
	}
	// mongo instance doesn't have db called golang-tests yet
	// but creating a collection will create golang-tests db
	testDB := mongoClient.Database("golang-tests")
	collectionNames, err := testDB.ListCollectionNames(context.TODO(), bson.D{})
	if err != nil {
		log.Fatal(err)
	}
	sessionCollFound := false
	userCollFound := false
	for _, collection := range collectionNames {
		if collection == "sessions" {
			sessionCollFound = true
		}
		if collection == "users" {
			userCollFound = true
		}
	}
	if !sessionCollFound {
		err = testDB.CreateCollection(context.TODO(), "sessions")
		if err != nil {
			log.Fatal(err)
		}
	}
	if !userCollFound {
		err = testDB.CreateCollection(context.TODO(), "users")
		if err != nil {
			log.Fatal(err)
		}
	}
	userCollection = mongoClient.Database("golang-tests").Collection("users")
	collections = append(collections, userCollection)
	sessionCollection = mongoClient.Database("golang-tests").Collection("sessions")
	collections = append(collections, sessionCollection)
}

func chainMiddleware(baseHandler http.Handler,
	middlewares ...func(http.Handler) http.Handler) http.Handler {
	for _, middleware := range middlewares {
		baseHandler = middleware(baseHandler)
	}
	return baseHandler
}

// first dummy route
func readCountAuthedUsers(w http.ResponseWriter, r *http.Request) {
	// set content type to json; what is being written back as a response
	// for server sent events, toggle this based on the api route frontend hits
	w.Header().Set("Content-Type", "application/json")
	authUserCount, err := sessionCollection.CountDocuments(context.TODO(), bson.D{})
	if err != nil {
		panic(err)
	}
	json.NewEncoder(w).Encode(fmt.Sprintf("hello mongo %d", authUserCount))
}

func main() {
	router := mux.NewRouter()
	// CORS access for frontend running on port 3000
	// use http://localhost:3000 for testing
	router.Use(handlers.CORS(handlers.AllowedOrigins([]string{"*"})))

	apiV1Router := router.PathPrefix("/api/v1").Subrouter()
	v1AuthRouter := apiV1Router.PathPrefix("/auth").Subrouter()
	v1ContentRouter := apiV1Router.PathPrefix("/content").Subrouter()

	v1AuthRouter.Handle("/register", auth.Register(collections...)).Methods("POST")
	v1AuthRouter.Handle("/login", auth.Login(collections...)).Methods("POST")

	v1ContentRouter.
		// type http.HandlerFunc implements serveHTTP method;
		// can be passed in when parameter expected to implement http.Handler interface
		Handle("/chain-test",
			chainMiddleware(http.HandlerFunc(readCountAuthedUsers),
				auth.AuthMiddleware(sessionCollection))).
		Methods("GET")

	log.Fatal(http.ListenAndServe(":8080", router))
}
