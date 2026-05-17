// @title           Read Tracker API
// @version         1.0
// @description     API for tracking reading progress of books, manga, manhua, novels, and articles.
// @host            localhost:8080
// @BasePath        /v1

package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"
	swaggerFiles "github.com/swaggo/files"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/erivelto/read-tracker/tracker/config"
	_ "github.com/erivelto/read-tracker/tracker/docs"
	"github.com/erivelto/read-tracker/tracker/handler"
	"github.com/erivelto/read-tracker/tracker/repository"
	"github.com/erivelto/read-tracker/tracker/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		logger.Error("failed to connect to MongoDB", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err = client.Disconnect(context.Background()); err != nil {
			logger.Error("failed to disconnect from MongoDB", "error", err)
		}
	}()

	if err = client.Ping(ctx, nil); err != nil {
		logger.Error("MongoDB ping failed", "error", err)
		os.Exit(1)
	}

	titlesCol := client.Database(cfg.DBName).Collection("titles")

	// Create a unique ascending index on the name field to enforce uniqueness at the database level.
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	if _, err = titlesCol.Indexes().CreateOne(ctx, indexModel); err != nil {
		logger.Error("failed to create titles index", "error", err)
		os.Exit(1)
	}

	titleRepo := repository.NewMongoTitleRepository(titlesCol)
	titleUC := usecase.NewTitleUsecase(titleRepo)
	titleHandler := handler.NewTitleHandler(titleUC)

	router := gin.New()
	router.Use(gin.Recovery())

	// Health endpoint for container healthchecks
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := router.Group("/v1")
	titleHandler.RegisterRoutes(v1)

	logger.Info("tracker api starting", "port", cfg.Port)
	if err = router.Run(":" + cfg.Port); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
