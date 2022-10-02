package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	CONNECTIONSTRING = "mongodb://localhost:27017"
	SQUARES          = "squares"
	LAST_TIMESTAMPS  = "last_timestamps"
)

func ConnectMongo() *mongo.Client {
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	uri := fmt.Sprintf("mongodb://%s:%s/%s", os.Getenv("MONGO_ADDRESS"), os.Getenv("MONGO_PORT"), os.Getenv("MONGO_DATABASE"))
	credential := options.Credential{
		AuthSource: os.Getenv("MONGO_AUTH_SOURCE"),
		Username:   os.Getenv("MONGO_USERNAME"),
		Password:   os.Getenv("MONGO_PASSWORD"),
	}

	client, err := mongo.NewClient(options.Client().ApplyURI(uri).SetAuth(credential))
	if err != nil {
		log.Fatalf("Cannot create mongo client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = client.Connect(ctx)
	if err != nil {
		log.Fatalf("Cannot connect to mongo server: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Cannot ping mongo server: %v", err)
	} else {
		log.Printf("connected to db at %s\n", uri)
	}
	return client
}

var Client *mongo.Client = ConnectMongo()

func GetCollection(client *mongo.Client, collectionName string) *mongo.Collection {
	return client.Database(os.Getenv("MONGO_DATABASE")).Collection(collectionName)
}
