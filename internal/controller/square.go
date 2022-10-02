package controllers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	log "github.com/sirupsen/logrus"

	"github.com/evandejesus/place-api/internal/cache"
	"github.com/evandejesus/place-api/internal/db"
	"github.com/evandejesus/place-api/internal/helpers"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	CANVAS_SIZE = 10
)

type Square struct {
	X         int    `json:"x" bson:"x" binding:"gte=0"`
	Y         int    `json:"y" bson:"y" binding:"gte=0"`
	Color     byte   `json:"color" bson:"color" binding:"required,lt=16"`
	Author    string `json:"author" bson:"author" binding:"required"`
	Timestamp int64  `json:"timestamp,omitempty" bson:"timestamp"`
}

var squareCollection *mongo.Collection = db.GetCollection(db.Client, "squares")
var timestampsCollection *mongo.Collection = db.GetCollection(db.Client, "last_timestamps")
var validate = validator.New()

// getSquares responds with the list of all squares as JSON.
func GetSquares(c *gin.Context) {

	// find all squares
	cursor, err := squareCollection.Find(context.TODO(), bson.D{})
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
func GetCanvas(c *gin.Context) {
	ctx := context.Background()
	redisCache := cache.GetRedisClient()

	const ARRAY_LENGTH = CANVAS_SIZE * CANVAS_SIZE
	bytes, err := redisCache.GetRange(ctx, fmt.Sprintf("squares-%d", CANVAS_SIZE), 0, ARRAY_LENGTH).Bytes()
	if helpers.Error(c, err) {
		log.Println(err)
		return
	}
	c.JSON(http.StatusOK, bytes)
}

// getSquareByLocation returns a single square given X and Y coordinates.
func GetSquareByLocation(c *gin.Context) {
	X, _ := strconv.Atoi(c.Query("X"))
	Y, _ := strconv.Atoi(c.Query("Y"))

	filter := bson.D{
		{Key: "X", Value: X},
		{Key: "Y", Value: Y},
	}
	var square Square
	err := squareCollection.FindOne(context.TODO(), filter).Decode(&square)
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

// putSquare creates a square object in the db.
func PutSquare(c *gin.Context) {
	redisCache := cache.GetRedisClient()

	var json Square
	if err := c.ShouldBind(&json); helpers.Error(c, err) {
		log.Println(err)
		return
	}

	// Validate request
	if err := validate.Struct(json); err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"status": false, "message": err.Error()})
		return
	}

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
	squareResult, err := squareCollection.UpdateOne(context.TODO(), filter, update, opts)
	if helpers.Error(c, err) {
		return
	}
	log.Debug(squareResult)
	// create timestamp entry
	timestampInsert := bson.D{{Key: "Author", Value: json.Author}, {Key: "Timestamp", Value: timestamp}}
	timestampResult, err := timestampsCollection.InsertOne(context.TODO(), timestampInsert)
	if helpers.Error(c, err) {
		return
	}
	log.Debug(timestampResult)

	redisResult := redisCache.BitField(context.TODO(), fmt.Sprintf("squares-%d", CANVAS_SIZE), "SET", "u4", fmt.Sprintf("#%d", json.X+CANVAS_SIZE*json.Y), json.Color)
	log.Debug(redisResult)

	c.JSON(http.StatusOK, map[string]interface{}{"status": true, "message": fmt.Sprintf("user %s successfully inserted color %d at pos (%d,%d)", json.Author, json.Color, json.X, json.Y)})
}
