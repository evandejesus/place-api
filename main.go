package main

import (
	"context"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/evandejesus/place-api/internal/connectionhelper"
)

var clientInstance *mongo.Client

type Square struct {
	X      int    `bson:"x"`
	Y      int    `bson:"y"`
	Color  byte   `bson:"color"`
	Author string `bson:"author"`
}

func main() {
	router := gin.Default()
	router.GET("/squares", getSquares)
	router.GET("/square", getSquareByLocation)
	router.PUT("/squares", putSquare)

	// DB
	clientInstance = connectionhelper.GetMongoClient()

	router.Run("localhost:8080")
}

// getSquares responds with the list of all squares as JSON.
func getSquares(c *gin.Context) {
	collection := clientInstance.Database(connectionhelper.DB).Collection(connectionhelper.SQUARES)
	cursor, err := collection.Find(context.TODO(), bson.D{})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	var results []Square
	if err = cursor.All(context.TODO(), &results); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, results)
}

func getSquareByLocation(c *gin.Context) {
	X, _ := strconv.Atoi(c.Query("X"))
	Y, _ := strconv.Atoi(c.Query("Y"))
	collection := clientInstance.Database(connectionhelper.DB).Collection(connectionhelper.SQUARES)
	filter := bson.D{
		{Key: "X", Value: X},
		{Key: "Y", Value: Y},
	}
	log.Println("filter:", filter)
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

	collection := clientInstance.Database(connectionhelper.DB).Collection(connectionhelper.SQUARES)
	filter := bson.D{
		{Key: "X", Value: json.X},
		{Key: "Y", Value: json.Y},
	}
	opts := options.Update().SetUpsert(true)

	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "X", Value: json.X},
		{Key: "Y", Value: json.Y},
		{Key: "Color", Value: json.Color},
		{Key: "Author", Value: json.Author},
	}}}
	result, err := collection.UpdateOne(context.TODO(), filter, update, opts)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
