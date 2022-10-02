package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/evandejesus/place-api/internal/db"
	"github.com/evandejesus/place-api/internal/helpers"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var squareCollection *mongo.Collection = db.GetCollection(db.Client, "squares")

// checkRateLimit throttles square creation to every TIMEOUT seconds.
func CheckRateLimit() gin.HandlerFunc {
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
		if helpers.Error(c, err) {
			return
		}

		opts := options.FindOne().SetProjection(bson.D{{Key: "Timestamp", Value: 1}}).SetSort(bson.D{{Key: "Timestamp", Value: -1}})
		var json struct {
			Timestamp int64 `bson:"timestamp"`
		}
		err = squareCollection.FindOne(context.TODO(), bson.D{{Key: "Author", Value: authorStruct.Author}}, opts).Decode(&json)
		// no timestamps exist with this user
		if err == mongo.ErrNoDocuments {
			c.Next()
			return
		} else if helpers.Error(c, err) {
			return // exit
		}

		// check if user has made any squares within the cooldown period, block request if so
		if time.Now().Unix()-json.Timestamp < int64(helpers.GetPutTimeout()) {
			log.Println("too many requests")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"status": false, "message": "too many requests"})
			return
		}
		c.Next()
	}
}
