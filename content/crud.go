package content

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type Item struct {
	Name           string `bson:"name"`
	Cost           int    `bson:"cost"`
	Classification string `bson:"classification"`
	Availability   bool   `bson:"availability"`
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
