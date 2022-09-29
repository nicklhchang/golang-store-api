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

func GetCartByUserSession(collections ...*mongo.Collection) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
	})
}

func GetMenuHandler(collections ...*mongo.Collection) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
