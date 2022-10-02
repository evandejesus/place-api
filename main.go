package main

import (
	"context"
	"errors"
	"fmt"
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

const (
	CANVAS_SIZE = 10
)

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
	router.GET("/canvas", getCanvas)

	// DB
	mongoDB = db.GetMongoClient(ctx)
	redisCache = cache.GetRedisClient()

	router.Run("localhost:8080")
}

// getSquares responds with the list of all squares as JSON.
func getSquares(c *gin.Context) {

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

// getCanvas returns the whole redis bitfield as a byte array.
func getCanvas(c *gin.Context) {
	ctx := context.Background()
	const ARRAY_LENGTH = CANVAS_SIZE * CANVAS_SIZE
	bytes, err := redisCache.GetRange(ctx, fmt.Sprintf("squares-%d", CANVAS_SIZE), 0, ARRAY_LENGTH).Bytes()
	if helpers.Error(c, err) {
		log.Println(err)
		return
	}
	c.JSON(http.StatusOK, bytes)
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
		c.JSON(http.StatusOK, Square{
			X:     X,
			Y:     Y,
			Color: 0,
		})

		return
	}
	c.JSON(http.StatusOK, square)

}

type NewSquare struct {
	X      int    `json:"x" binding:"required"`
	Y      int    `json:"y" binding:"required"`
	Color  byte   `json:"color" binding:"required"`
	Author string `json:"author" binding:"required"`
}

func (sq *NewSquare) Validate() error {
	if sq.Color >= 16 {
		return errors.New("color must be less than 16")
	}
	if sq.X < 0 || sq.Y < 0 {
		return fmt.Errorf("invalid coordinates: (%d,%d)", sq.X, sq.Y)
	}
	return nil
}

// putSquare creates a square object in the db.
func putSquare(c *gin.Context) {
	ctx := context.Background()

	var json NewSquare
	if err := c.ShouldBind(&json); helpers.Error(c, err) {
		log.Println(err)
		return
	}

	// Validate request
	if err := json.Validate(); err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": false, "message": err.Error()})
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
	// upsert square
	squareResult, err := squaresColl.UpdateOne(ctx, filter, update, opts)
	if helpers.Error(c, err) {
		return
	}
	log.Debug(squareResult)
	// create timestamp entry
	timestampInsert := bson.D{{Key: "Author", Value: json.Author}, {Key: "Timestamp", Value: timestamp}}
	timestampResult, err := timestampsColl.InsertOne(ctx, timestampInsert)
	if helpers.Error(c, err) {
		return
	}
	log.Debug(timestampResult)

	redisResult := redisCache.BitField(ctx, fmt.Sprintf("squares-%d", CANVAS_SIZE), "SET", "u4", fmt.Sprintf("#%d", json.X+CANVAS_SIZE*json.Y), json.Color)
	log.Debug(redisResult)

	c.JSON(http.StatusOK, map[string]interface{}{"status": true, "message": fmt.Sprintf("user %s successfully inserted color %d at pos (%d,%d)", json.Author, json.Color, json.X, json.Y)})
}
