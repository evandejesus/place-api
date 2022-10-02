package db

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	CONNECTIONSTRING = "mongodb://localhost:27017"
	SQUARES          = "squares"
	LAST_TIMESTAMPS  = "last_timestamps"
)

func GetMongoClient(ctx context.Context) *mongo.Client {

	uri := fmt.Sprintf("mongodb://%s:%s/%s", os.Getenv("MONGO_ADDRESS"), os.Getenv("MONGO_PORT"), os.Getenv("MONGO_DATABASE"))
	credential := options.Credential{
		AuthSource: os.Getenv("MONGO_AUTH_SOURCE"),
		Username:   os.Getenv("MONGO_USERNAME"),
		Password:   os.Getenv("MONGO_PASSWORD"),
	}

	clientOpts := options.Client().ApplyURI(uri).SetAuth(credential)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		log.Fatalf("Cannot connect to mongo server: %v", err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatalf("Cannot ping mongo server: %v", err)
	} else {
		log.Printf("connected to db at %s\n", uri)
	}
	return client
}

func Squares(db *mongo.Client) *mongo.Collection {
	return db.Database(os.Getenv("MONGO_DATABASE")).Collection(SQUARES)
}

func Timestamps(db *mongo.Client) *mongo.Collection {
	return db.Database(os.Getenv("MONGO_DATABASE")).Collection(LAST_TIMESTAMPS)
}
