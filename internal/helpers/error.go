package helpers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Error gracefully handles errors and sends them to user.
func Error(c *gin.Context, err error) bool {
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": true, "message": err.Error()})
		return true // signal that there was an error and the caller should return
	}
	return false // no error, can continue
}
