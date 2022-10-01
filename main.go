package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/evandejesus/place-api/internal/db"
	"github.com/evandejesus/place-api/internal/helpers"
)

var clientInstance *mongo.Client

type Square struct {
	X         int    `bson:"x"`
	Y         int    `bson:"y"`
	Color     byte   `bson:"color"`
	Author    string `bson:"author"`
	Timestamp int64  `bson:"timestamp"`
}

func main() {
	router := gin.New()
	router.Use(gin.Logger())

	router.GET("/squares", getSquares)
	router.GET("/square", getSquareByLocation)
	router.PUT("/squares", checkRateLimit(), putSquare)

	// DB
	clientInstance = db.GetMongoClient()

	router.Run("localhost:8080")
}

// getSquares responds with the list of all squares as JSON.
func getSquares(c *gin.Context) {
	collection := clientInstance.Database(db.DB).Collection(db.SQUARES)
	cursor, err := collection.Find(context.TODO(), bson.D{})
	if Error(c, err) {
		return
	}
	var results []Square
	err = cursor.All(context.TODO(), &results)
	if Error(c, err) {
		return
	}
	c.JSON(http.StatusOK, results)
}

func getSquareByLocation(c *gin.Context) {
	X, _ := strconv.Atoi(c.Query("X"))
	Y, _ := strconv.Atoi(c.Query("Y"))
	collection := clientInstance.Database(db.DB).Collection(db.SQUARES)
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

func putSquare(c *gin.Context) {

	var json struct {
		X      int    `json:"x" binding:"required"`
		Y      int    `json:"y" binding:"required"`
		Color  byte   `json:"color" binding:"required"`
		Author string `json:"author" binding:"required"`
	}

	if err := c.Bind(&json); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	collection := clientInstance.Database(db.DB).Collection(db.SQUARES)
	timestampCollection := clientInstance.Database(db.DB).Collection(db.LAST_TIMESTAMPS)

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
	result, err := collection.UpdateOne(context.TODO(), filter, update, opts)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	timestampInsert := bson.D{{Key: "Author", Value: json.Author}, {Key: "Timestamp", Value: timestamp}}
	timestampResult, err := timestampCollection.InsertOne(context.TODO(), timestampInsert)
	log.Println(timestampResult.InsertedID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func checkRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {

		// move request body to tmp variable
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
		}

		// restore the io.ReadCloser to its original state
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// extract author from body
		var authorStruct struct {
			Author string `json:"author" binding:"required"`
		}
		err := json.Unmarshal(bodyBytes, &authorStruct)
		if Error(c, err) {
			return
		}

		collection := clientInstance.Database(db.DB).Collection(db.LAST_TIMESTAMPS)
		opts := options.FindOne().SetProjection(bson.D{{Key: "Timestamp", Value: 1}}).SetSort(bson.D{{Key: "Timestamp", Value: -1}})
		var json struct {
			Timestamp int64 `bson:"timestamp"`
		}
		err = collection.FindOne(context.TODO(), bson.D{{Key: "Author", Value: authorStruct.Author}}, opts).Decode(&json)
		if err == mongo.ErrNoDocuments {
			c.Next()
			return
		} else if Error(c, err) {
			return // exit
		}
		if time.Now().Unix()-json.Timestamp < int64(helpers.PUT_TIMEOUT) {
			log.Println("too many requests")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"status": false, "message": "too many requests"})
			return
		}
		c.Next()
	}
}

func Error(c *gin.Context, err error) bool {
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"status": false, "message": err.Error()})
		return true // signal that there was an error and the caller should return
	}
	return false // no error, can continue
}
