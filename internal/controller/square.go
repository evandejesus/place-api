package controllers

import (
	"context"
	"errors"
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
	CANVAS_SIZE = 3
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
	c.JSON(http.StatusOK, helpers.UserResponse{
		Error: false,
		Data:  results,
	})
}

// getCanvas returns the whole redis bitfield as a byte array.
func GetCanvas(c *gin.Context) {
	ctx := context.Background()
	redisCache := cache.GetRedisClient()

	const ARRAY_LENGTH = CANVAS_SIZE * CANVAS_SIZE
	result := redisCache.Get(ctx, fmt.Sprintf("squares-%d", CANVAS_SIZE))
	if result.Err() != nil {
		c.JSON(http.StatusOK, make([]byte, ARRAY_LENGTH))
		return
	}
	bytes, err := result.Bytes()
	log.Debug("bytes: ", bytes)
	newBytes := make([]byte, ARRAY_LENGTH)
	copy(newBytes, bytes)
	if helpers.Error(c, err) {
		log.Println(err)
		return
	}
	log.Info("newBytes: ", newBytes)
	c.JSON(http.StatusOK, helpers.UserResponse{
		Error: false,
		Data:  newBytes,
	})
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
		c.JSON(http.StatusOK, helpers.UserResponse{
			Error: false,
			Data: Square{
				X:     X,
				Y:     Y,
				Color: 0,
			},
		})

		return
	}

	c.JSON(http.StatusOK, helpers.UserResponse{
		Error: false,
		Data:  square,
	})
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

	if json.X >= CANVAS_SIZE || json.Y >= CANVAS_SIZE {
		err := errors.New("invalid coordinate")
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
	_, err := squareCollection.UpdateOne(context.TODO(), filter, update, opts)
	if helpers.Error(c, err) {
		return
	}
	// create timestamp entry
	timestampInsert := bson.D{{Key: "Author", Value: json.Author}, {Key: "Timestamp", Value: timestamp}}
	_, err = timestampsCollection.InsertOne(context.TODO(), timestampInsert)
	if helpers.Error(c, err) {
		return
	}

	// set bitfield in redis
	redisResult := redisCache.BitField(context.TODO(), fmt.Sprintf("squares-%d", CANVAS_SIZE), "SET", "u8", fmt.Sprintf("#%d", json.X+CANVAS_SIZE*json.Y), json.Color)
	log.Trace(redisResult)
	c.JSON(http.StatusOK, helpers.UserResponse{
		Error: false,
		Data:  fmt.Sprintf("user %s successfully inserted color %d at pos (%d,%d)", json.Author, json.Color, json.X, json.Y),
	})
}
