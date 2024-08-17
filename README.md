# SurgeLimit Go

SurgeLimit Go is a flexible and efficient rate limiting package for Go, designed to work with a variety of caching systems, including Redis, RedisCluster, ValKey, KeyDB, DragonflyDB, and Kvrocks. It implements a leaky bucket algorithm to help you control the rate of requests and manage traffic surges effectively.

## Features

- **Leaky Bucket Algorithm**: Implements the leaky bucket rate limiting algorithm, which smooths out bursts of traffic by allowing a fixed rate of requests and handling excess traffic gracefully.

- **Multi-System Support**: Compatible with multiple redis compatible systems, providing flexibility to integrate with your existing infrastructure.

- **High Performance**: Designed for high performance and low latency, making it suitable for high-throughput applications.

- **Configurable Limits**: Easily configure rate limits based on your application's needs, including maximum request rates and burst capacities.

- **Redis Integration**: Fully integrated with popular Redis-based systems, offering robust support for distributed environments.

## Installation

To install SurgeLimit Go, use the following command:

```bash
go get github.com/driftdev/surgelimit-go
```
