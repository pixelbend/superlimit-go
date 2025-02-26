# SurgeLimit Go

SurgeLimit Go is a flexible and efficient rate limiting package for Go, designed to work with a variety of caching systems, including Redis, RedisCluster, ValKey, KeyDB, DragonflyDB, and Kvrocks. It implements a leaky bucket algorithm to help you control the rate of requests and manage traffic surges effectively.

## Features

- **Leaky Bucket Algorithm**: Implements the leaky bucket rate limiting algorithm, which smooths out bursts of traffic by allowing a fixed rate of requests and handling excess traffic gracefully.

- **Multi-System Support**: Compatible with multiple Redis compatible systems, providing flexibility to integrate with your existing infrastructure.

- **High Performance**: Designed for high performance and low latency, making it suitable for high-throughput applications.

- **Configurable Limits**: Easily configure rate limits based on your application's needs, including maximum request rates and burst capacities.

- **Redis Integration**: Fully integrated with popular Redis-based systems, offering robust support for distributed environments.

## Installation

To install SurgeLimit Go, use the following command.

```bash
go get github.com/pixelbend/surgelimit-go
```

## Usage

Here's a basic example of how to use SurgeLimit Go.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/pixelbend/surgelimit-go/leakybucket"
	"github.com/go-redis/redis/v9"
)

func main() {
	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", 
		Password: "",               
		DB:       0,                
	})

	// Flush the Redis database to start with a clean state
	if err := rdb.FlushDB(context.Background()).Err(); err != nil {
		log.Fatalf("Error flushing DB: %v", err)
	}

	// Create a new leaky bucket rate limiter with default options
	limiter := leakybucket.NewLimiter(rdb, leakybucket.DefaultOptions())

	// Attempt to allow a request under the rate limit
	res, err := limiter.Allow(context.Background(), "project:01J61AAPTXV3HQD95XQWATPBS8", leakybucket.LimitPerSecond(10))
	if err != nil {
		log.Fatalf("Error checking rate limit: %v", err)
	}

	// Check if the request is allowed
	if res.Allowed > 0 {
		// If allowed, proceed with the application logic
		log.Println("Request allowed, proceeding with application logic...")
	} else {
		// If not allowed, handle the rate limit exceeded case
		log.Println("Request denied: rate limit exceeded")
	}
}
```
