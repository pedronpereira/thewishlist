package storage

import (
	"context"
	"fmt"

	"github.com/pedronpereira/thewishlist/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoCloudStore struct {
	uri        string
	dbName     string
	collection string
}

func (cs *MongoCloudStore) Load() domain.Wishlist {
	fmt.Println("Loading data from the Cloud")

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(cs.uri))
	if err != nil {
		fmt.Printf("ERROR %s: %v", "Connecting to database", err)
		panic(err)
	}

	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database(cs.dbName).Collection(cs.collection)
	//TODO: change this id for the collection to get all data in the collection.
	filter := bson.D{{Key: "_id", Value: "12333312-4123-1222-b15b-ea7a8b1de6fd"}}
	var payload domain.Wishlist
	err = coll.FindOne(context.Background(), filter).Decode(&payload)
	if err != nil {
		fmt.Printf("ERROR %s: %v", "Connecting to database", err)
		panic(err)
	}

	return payload
}

func (cs *MongoCloudStore) SaveWishList(payload domain.Wishlist) error {

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(cs.uri))
	if err != nil {
		fmt.Printf("ERROR %s: %v", "Connecting to database", err)
		panic(err)
	}

	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()

	coll := client.Database(cs.dbName).Collection(cs.collection)
	//TODO: change this id for the collection to get all data in the collection.
	filter := bson.D{{Key: "_id", Value: "12333312-4123-1222-b15b-ea7a8b1de6fd"}}

	_, err = coll.ReplaceOne(context.Background(), filter, payload)
	return err
}
