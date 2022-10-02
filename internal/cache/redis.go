package cache

import (
	"os"

	"github.com/go-redis/redis/v8"
)

func GetRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDRESS"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0, // use default DB,
	})
}
