package main

import (
	"github.com/gin-gonic/gin"

	docs "github.com/evandejesus/place-api/docs"
	controller "github.com/evandejesus/place-api/internal/controller"
	"github.com/evandejesus/place-api/internal/db"
	"github.com/evandejesus/place-api/internal/middleware"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           Place API
// @version         1.0
// @description     This is an implementation of r/place using Go.

// @contact.name   Evan
// @contact.email  evanjdejesus@gmail.com

// @license.name MIT License

// @host      localhost:8080
// @BasePath  /api
func main() {

	router := gin.New()
	docs.SwaggerInfo.BasePath = "/api"
	router.SetTrustedProxies(nil)
	router.Use(gin.Logger())

	api := router.Group("/api")
	{

		api.GET("/squares", controller.GetSquares)
		api.GET("/square", controller.GetSquareByLocation)
		api.PUT("/squares", middleware.CheckRateLimit(), controller.PutSquare)
		api.GET("/canvas", controller.GetCanvas)
	}

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	db.ConnectMongo()

	router.Run("localhost:8080")
}
