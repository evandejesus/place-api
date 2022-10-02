package main

import (
	"context"
	"net/http"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"

	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/evandejesus/place-api/internal/cache"
	"github.com/evandejesus/place-api/internal/db"
	"github.com/evandejesus/place-api/internal/helpers"
	"github.com/evandejesus/place-api/internal/middleware"
)

var mongoDB *mongo.Client
var redisCache *redis.Client

type Square struct {
	X         int    `bson:"x"`
	Y         int    `bson:"y"`
	Color     byte   `bson:"color"`
	Author    string `bson:"author"`
	Timestamp int64  `bson:"timestamp"`
}

func main() {
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	ctx := context.Background()
	router := gin.New()
	router.SetTrustedProxies(nil)
	router.Use(gin.Logger())

	router.GET("/squares", getSquares)
	router.GET("/square", getSquareByLocation)
	router.PUT("/squares", middleware.CheckRateLimit(), putSquare)

	// DB
	mongoDB = db.GetMongoClient(ctx)
	redisCache = cache.GetRedisClient()

	router.Run("localhost:8080")
}

// getSquares responds with the list of all squares as JSON.
func getSquares(c *gin.Context) {
	// ctx := context.Background()
	// redisCache.Set(ctx, "key", "val2", 0)
	// val, _ := redisCache.Get(ctx, "key").Result()
	// log.Println(val)

	// find all squares
	collection := db.Squares(mongoDB)
	cursor, err := collection.Find(context.TODO(), bson.D{})
	if helpers.Error(c, err) {
		return
	}

	// iterate over list of documents
	var results []Square
	err = cursor.All(context.TODO(), &results)
	if helpers.Error(c, err) {
		return
	}
	c.JSON(http.StatusOK, results)
}

// getSquareByLocation returns a single square given X and Y coordinates.
func getSquareByLocation(c *gin.Context) {
	X, _ := strconv.Atoi(c.Query("X"))
	Y, _ := strconv.Atoi(c.Query("Y"))
	collection := db.Squares(mongoDB)
	filter := bson.D{
		{Key: "X", Value: X},
		{Key: "Y", Value: Y},
	}
	var square Square
	err := collection.FindOne(context.TODO(), filter).Decode(&square)
	if err == mongo.ErrNoDocuments {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, square)

}

// putSquare creates a square object in the db.
func putSquare(c *gin.Context) {

	var json struct {
		X      int    `json:"x" binding:"required"`
		Y      int    `json:"y" binding:"required"`
		Color  byte   `json:"color" binding:"required"`
		Author string `json:"author" binding:"required"`
	}

	if err := c.Bind(&json); helpers.Error(c, err) {
		return
	}

	squaresColl := db.Squares(mongoDB)
	timestampsColl := db.Timestamps(mongoDB)

	/*
		This function creates a square object if none exists at the given coordinate.
		Otherwise, updates all provided fields.
	*/
	filter := bson.D{
		{Key: "X", Value: json.X},
		{Key: "Y", Value: json.Y},
	}
	opts := options.Update().SetUpsert(true)
	timestamp := time.Now().Unix()
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "X", Value: json.X},
		{Key: "Y", Value: json.Y},
		{Key: "Color", Value: json.Color},
		{Key: "Author", Value: json.Author},
		{Key: "Timestamp", Value: timestamp},
	}}}
	result, err := squaresColl.UpdateOne(context.TODO(), filter, update, opts)
	if helpers.Error(c, err) {
		return
	}
	timestampInsert := bson.D{{Key: "Author", Value: json.Author}, {Key: "Timestamp", Value: timestamp}}
	timestampResult, err := timestampsColl.InsertOne(context.TODO(), timestampInsert)
	if helpers.Error(c, err) {
		return
	}
	c.JSON(http.StatusOK, map[string]interface{}{"squares": result, "timestamps": timestampResult})
}
