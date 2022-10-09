package content

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

/*
session is valid because auth.AuthMiddleware would have terminated otherwise.
auth.AuthMiddleware still needed because Carts collection will save carts for a
User's old sessions. so auth.AuthMiddleware essentially validates session validity,
even though looking up a Cart in Carts collection may still find a record.

because context is cancelled by ServeHTTP, cannot pass user and session to here,
will just have to look up in Carts collection with the session-id in cookies.
documents in Carts collection have unique user and session as well as the cart
*/
func GetCartByUserSession(collections ...*mongo.Collection) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ptrCookieSlice := r.Cookies()
		var sessionID string
		for _, ptrCookie := range ptrCookieSlice {
			if (*ptrCookie).Name == "session-id" {
				sessionID = (*ptrCookie).Value
			}
		}
		cartForUserSession, err := GetCart(sessionID, collections[1])
		if err != nil {
			fmt.Printf("let's investigate why search in db failed: %v", err)
		}
		if len(cartForUserSession) == 0 {
			json.NewEncoder(w).Encode("no cart for user session combo")
			return
		}
		// slice of bson.M's which type casted to map[string]interface{}
		// var cartItemsAsSlice []map[string]interface{}
		json.NewEncoder(w).Encode(fmt.Sprintf("cart for user's current session: %v\n", cartForUserSession))
	})
}

func PutUpsertCartSync(collections ...*mongo.Collection) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ptrCookieSlice := r.Cookies()
		filter := bson.D{}
		for _, ptrCookie := range ptrCookieSlice {
			if (*ptrCookie).Name == "session-id" {
				filter = append(filter, bson.E{Key: "session", Value: (*ptrCookie).Value})
			}
		}
		// so far not handling any request body from post req
		err := UpsertCart(filter, collections[1])
		if err != nil {
			fmt.Printf("here's the error when upserting cart: %v", err)
		}
	})
}

func GetMenuHandler(collections ...*mongo.Collection) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// fmt.Printf("request context: %v\n", r.Context())
		w.Header().Set("Content-Type", "application/json")
		// required to get request URL params
		err := r.ParseForm()
		if err != nil {
			log.Fatal(err)
		}
		// could iterate over r.Form.Get("types") and check against ASCII code for ,
		types := strings.Split(r.Form.Get("types"), ",")
		// extract prive but convert to integer base 10 for mongo api
		price, err := strconv.Atoi(r.Form.Get("price"))
		if err != nil {
			json.NewEncoder(w).Encode("price provided could not be converted to integer")
		}

		// done in this format following MongoDB Go Driver docs
		filter := bson.D{
			{Key: "cost", Value: bson.D{{Key: "$lte", Value: price}}},
			{Key: "classification", Value: bson.D{{Key: "$in", Value: types}}},
		}
		// could use append and replace each element of bson.D with bson.E (E for element)
		// filter = append(filter, bson.E{Key: "cost", Value: bson.D{{Key: "$lte", Value: price}}})
		filter = append(filter, bson.E{Key: "availability", Value: true})

		// use a crud function for readability
		items, err := GetMenu(filter, collections[0])
		if err != nil {
			fmt.Printf("let's inspect items: %v and error: %v", items, err)
		}
		json.NewEncoder(w).Encode(items)
	})
}
