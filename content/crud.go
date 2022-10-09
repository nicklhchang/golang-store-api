package content

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// blueprints to give result variable a type,
// so mongo results can be decoded easily into result variable
type User struct {
	Name string `bson:"user"`
	Pwd  string `bson:"pwd"`
}

type Session struct {
	User    string `bson:"user"` // not User.Name because User.Name not defined as a type
	Session string `bson:"session"`
}

type Item struct {
	Name           string `bson:"name"`
	Cost           int    `bson:"cost"`
	Classification string `bson:"classification"`
	Availability   bool   `bson:"availability"`
}

type Cart struct {
	Items      []Item `bson:"items"`
	User       string `bson:"user"`
	Session    string `bson:"session"`
	LastUpdate int64  `bson:"lastUpdate"`
}

func GetCart(sessionID string, cCollection *mongo.Collection) (map[string]interface{}, error) {
	findCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var result bson.M
	err := cCollection.FindOne(findCtx, bson.D{{Key: "session", Value: sessionID}}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return map[string]interface{}{}, nil // empty map
		}
		log.Fatal(err)
		return nil, err
	}
	return result, nil // bson.M just fancy wrapping for map[string]interface{}
}

func UpsertCart(filter bson.D, cCollection *mongo.Collection) error {
	// considering that upsert may create new document will need to put req body
	// into bson.D and then pass as param to SetUpdate(), along with $set lastUpdate.
	timeAtUpsert := time.Now().Unix()
	timestamp := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "lastUpdate", Value: timeAtUpsert},
		}},
	}
	bwModelSlice := []mongo.WriteModel{
		mongo.NewUpdateOneModel().SetFilter(filter).
			SetUpdate(timestamp).SetUpsert(true),
	}
	_, err := cCollection.BulkWrite(context.TODO(), bwModelSlice)
	if err != nil {
		fmt.Printf("couldn't update cart document %v successfully: %v", filter, err)
		return err
	}
	return nil
}

// return anything for now
func GetMenu(filter bson.D, iCollection *mongo.Collection) ([]Item, error) {
	// notice how a context is returned by WithTimeout() and first parameter is context too
	// can chain contexts and add deadlines, cancellation channels, timeouts other info
	findCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resultCursor, err := iCollection.Find(findCtx, filter)
	if err != nil {
		fmt.Printf("get menu mongo api Find() failed: %v", err)
		return nil, err
	}
	// not expecting more than 50 items in collection right now
	retItems := make([]Item, 0, 50)
	// context.Background; no initial deadline, no initial cancellation (Done) channel
	cursCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	for another := resultCursor.Next(cursCtx); another; another = resultCursor.Next(cursCtx) {
		var item Item
		err = resultCursor.Decode(&item)
		if err != nil {
			fmt.Printf("issues decoding current cursor doc into go value: %v", err)
			return nil, err
		}
		retItems = append(retItems, item)
	}
	return retItems, nil
}
