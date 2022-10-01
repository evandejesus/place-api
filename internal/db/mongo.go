package db

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

const (
	CONNECTIONSTRING = "mongodb://localhost:27017"
	DB               = "place"
	SQUARES          = "squares"
	LAST_TIMESTAMPS  = "last_timestamps"
)

func GetMongoClient() *mongo.Client {
	credential := options.Credential{
		AuthSource: "admin",
		Username:   "place",
		Password:   "place",
	}

	clientOpts := options.Client().ApplyURI("mongodb://localhost:27017/" + DB).SetAuth(credential)
	client, err := mongo.Connect(context.TODO(), clientOpts)
	if err != nil {
		panic(err)
	}
	if err := client.Ping(context.TODO(), readpref.Primary()); err != nil {
		panic(err)
	}
	return client
}