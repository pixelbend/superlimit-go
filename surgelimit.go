package surgelimit

import (
	"context"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
)

const defaultKeyPrefix = "RATE_LIMIT"

// Options holds the configuration settings for the Limiter.
type Options struct {
	// KeyPrefix is the prefix added to all keys used by the limiter.
	// This helps to distinguish rate-limiting keys from other keys.
	// By default, it is set to "RATE_LIMIT", but it can be customized to avoid key collisions
	KeyPrefix string
}

// DefaultOptions returns an Options struct populated with the default settings
// for the Limiter. Currently, this includes setting the `KeyPrefix` to "RATE_LIMIT".
//
// Example:
//
//	options := DefaultOptions()
//	// Creates options with KeyPrefix set to "RATE_LIMIT"
func DefaultOptions() Options {
	return Options{
		KeyPrefix: defaultKeyPrefix,
	}
}

// Limiter is the main struct that handles rate limiting. It uses a Redis client
// to store and retrieve rate-limiting information and operates based on the
// provided Options.
//
// Fields:
//   - client: A Redis client used to interact with the Redis or Redis Compatible database.
//   - Options: Configuration options for the Limiter
type Limiter struct {
	client  redis.UniversalClient
	Options Options
}

// NewLimiter creates and returns a new Limiter instance using the provided
// Redis client and options. The Limiter is responsible for managing rate limits
// based on the given configuration.
//
// Parameters:
//   - client: A Redis UniversalClient used to interact with the Redis database.
//   - options: An Options struct that configures the Limiter, including the key prefix.
//
// Returns:
//   - *Limiter: A pointer to the newly created Limiter instance.
//
// Example:
//
//	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
//	    // Redis client configuration
//	})
//
//	// Create a limiter with default options
//	limiter := NewLimiter(redisClient, DefaultOptions())
//
//	// Or create a limiter with custom options
//	customOptions := Options{KeyPrefix: "CUSTOM_PREFIX"}
//	limiter := NewLimiter(redisClient, customOptions)
func NewLimiter(client redis.UniversalClient, options Options) *Limiter {
	return &Limiter{
		client:  client,
		Options: options,
	}
}

// Allow attempts to allow a single request for a given key under a rate-limiting
// scheme defined by the `surgelimit.Limit` struct. This is a convenience method that
// calls `AllowN` with `n` set to 1, meaning it checks if just one request can
// be made at the current time.
//
// Parameters:
//   - ctx: The context to control cancellation and timeouts.
//   - key: The unique identifier for the entity being rate-limited (e.g., user ID, IP address).
//   - limit: A `surgelimit.Limit` struct defining the rate (requests per period) and burst (maximum burst size).
//     This can be created using helper functions such as `surgelimit.PerSecond`, `surgelimit.PerMinute`, or `surgelimit.PerHour`.
//
// Returns:
//   - *Result: A struct containing information about the rate limiting state, such as the number
//     of requests allowed, remaining, and times for retry and reset.
//   - error: If an error occurs while executing the underlying `AllowN` method or parsing the result, it is returned.
//
// Example:
//
//	// Create a rate limit of 5 requests per second
//	limit := surgelimit.PerSecond(5)
//
//	result, err := Allow(ctx, "user_1234", limit)
//	if err != nil {
//	    log.Fatalf("Failed to check rate limit: %v", err)
//	}
//
//	if result.Allowed > 0 {
//	    log.Println("Request allowed.")
//	} else {
//	    log.Printf("Rate limit exceeded. Retry after %v, reset after %v.", result.RetryAfter, result.ResetAfter)
//	}
func (l *Limiter) Allow(ctx context.Context, key string, limit Limit) (*Result, error) {
	return l.AllowN(ctx, key, limit, 1)
}

// AllowN attempts to allow `n` requests for a given key under a rate-limiting
// scheme defined by the `Limit` struct. This method uses a Lua script to
// calculate whether the requests can be allowed based on the current rate limit state.
//
// The `surgelimit.Limit` struct can be easily created using helper functions like `surgelimit.PerSecond`,
// `surgelimit.PerMinute`, and `surgelimit.PerHour`, which define the rate limit in terms of requests
// per second, minute, or hour respectively. Each helper function sets the `Burst`
// to be equal to the `Rate`, meaning that the system can handle a burst of up to
// `Rate` requests in one period.
//
// Parameters:
//   - ctx: The context to control cancellation and timeouts.
//   - key: The unique identifier for the entity being rate-limited (e.g., user ID, IP address).
//   - limit: A `surgelimit.Limit` struct defining the rate (requests per period) and burst (maximum burst size).
//   - n: The number of requests to attempt to allow.
//
// Returns:
//   - *Result: A struct containing information about the rate limiting state, such as the number
//     of requests allowed, remaining, and times for retry and reset.
//   - error: If an error occurs while executing the Lua script or parsing the result, it is returned.
//
// Example:
//
//	// Create a rate limit of 5 requests per second
//	limit := surgelimit.PerSecond(10)
//
//	result, err := AllowN(ctx, "user_1234", limit, 3)
//	if err != nil {
//	    log.Fatalf("Failed to check rate limit: %v", err)
//	}
//
//	if result.Allowed > 0 {
//	    log.Printf("Allowed %d requests, %d remaining.", result.Allowed, result.Remaining)
//	} else {
//	    log.Printf("Rate limit exceeded. Retry after %v, reset after %v.", result.RetryAfter, result.ResetAfter)
//	}
func (l *Limiter) AllowN(
	ctx context.Context,
	key string,
	limit Limit,
	n int,
) (*Result, error) {
	values := []interface{}{limit.Burst, limit.Rate, limit.Period.Seconds(), n}
	v, err := allowN.Run(ctx, l.client, []string{keyWithPrefix(l.Options.KeyPrefix, key)}, values...).Result()
	if err != nil {
		return nil, err
	}

	values = v.([]interface{})

	retryAfter, err := strconv.ParseFloat(values[2].(string), 64)
	if err != nil {
		return nil, err
	}

	resetAfter, err := strconv.ParseFloat(values[3].(string), 64)
	if err != nil {
		return nil, err
	}

	res := &Result{
		Limit:      limit,
		Allowed:    int(values[0].(int64)),
		Remaining:  int(values[1].(int64)),
		RetryAfter: dur(retryAfter),
		ResetAfter: dur(resetAfter),
	}
	return res, nil
}

// AllowAtMost attempts to allow up to `n` requests for a given key under a rate-limiting
// scheme defined by the `surgelimit.Limit` struct. This method is similar to `AllowN`, but with
// the focus on allowing the maximum possible number of requests up to the limit `n`
// without exceeding the rate limit.
//
// The `surgelimit.Limit` struct can be easily created using helper functions like `surgelimit.PerSecond`,
// `surgelimit.PerMinute`, and `surgelimit.PerHour`, which define the rate limit in terms of requests
// per second, minute, or hour respectively. Each helper function sets the `Burst`
// to be equal to the `Rate`, meaning that the system can handle a burst of up to
// `Rate` requests in one period.
//
// Parameters:
//   - ctx: The context to control cancellation and timeouts.
//   - key: The unique identifier for the entity being rate-limited (e.g., user ID, IP address).
//   - limit: A `surgelimit.Limit` struct defining the rate (requests per period) and burst (maximum burst size).
//     This can be created using helper functions such as `surgelimit.PerSecond`, `surgelimit.PerMinute`, or `surgelimit.PerHour`.
//   - n: The maximum number of requests to attempt to allow.
//
// Returns:
//   - *Result: A struct containing information about the rate limiting state, such as the number
//     of requests actually allowed, remaining, and times for retry and reset.
//   - error: If an error occurs while executing the Lua script or parsing the result, it is returned.
//
// Example:
//
//	// Create a rate limit of 10 requests per minute
//	limit := surgelimit.PerMinute(10)
//
//	// Attempt to allow up to 5 requests
//	result, err := AllowAtMost(ctx, "user_1234", limit, 5)
//	if err != nil {
//	    log.Fatalf("Failed to check rate limit: %v", err)
//	}
//
//	log.Printf("Allowed %d requests, %d remaining.", result.Allowed, result.Remaining)
//	if result.Allowed < 5 {
//	    log.Printf("Rate limit exceeded for some requests. Retry after %v, reset after %v.", result.RetryAfter, result.ResetAfter)
//	}
func (l *Limiter) AllowAtMost(
	ctx context.Context,
	key string,
	limit Limit,
	n int,
) (*Result, error) {
	values := []interface{}{limit.Burst, limit.Rate, limit.Period.Seconds(), n}
	v, err := allowAtMost.Run(ctx, l.client, []string{keyWithPrefix(l.Options.KeyPrefix, key)}, values...).Result()
	if err != nil {
		return nil, err
	}

	values = v.([]interface{})

	retryAfter, err := strconv.ParseFloat(values[2].(string), 64)
	if err != nil {
		return nil, err
	}

	resetAfter, err := strconv.ParseFloat(values[3].(string), 64)
	if err != nil {
		return nil, err
	}

	res := &Result{
		Limit:      limit,
		Allowed:    int(values[0].(int64)),
		Remaining:  int(values[1].(int64)),
		RetryAfter: dur(retryAfter),
		ResetAfter: dur(resetAfter),
	}
	return res, nil
}

// Reset clears the rate limiter's state for a given key, effectively resetting
// any rate limiting associated with that key. This can be useful if you want to
// manually clear the rate limits for a specific user or action, for example,
// after a penalty period has passed or after a successful manual intervention.
//
// The method removes the key from the underlying storage, which
// effectively resets the rate limiting data (e.g., the number of requests made
// and the timestamps) for that key.
//
// Parameters:
//   - ctx: The context to control cancellation and timeouts.
//   - key: The unique identifier for the entity whose rate limit is to be reset.
//
// Returns:
//   - error: If an error occurs during the deletion process, it is returned.
//     If the operation is successful, it returns nil.
//
// Example:
//
//	// Reset the rate limit for user ID 1234
//	err := Reset(ctx, "user_1234")
//	if err != nil {
//		log.Printf("Warning: Failed to reset rate limiter for user_1234: %v", err)
//	}
func (l *Limiter) Reset(ctx context.Context, key string) error {
	return l.client.Del(ctx, keyWithPrefix(l.Options.KeyPrefix, key)).Err()
}

// dur converts a floating-point number representing seconds into a time.Duration.
// If the input is -1, the function returns -1, indicating an indefinite or unbounded duration.
func dur(f float64) time.Duration {
	if f == -1 {
		return -1
	}
	return time.Duration(f * float64(time.Second))
}

// keyWithPrefix creates a key by appending a specified key to a given prefix.
// This helps to ensure that the keys used for rate limiting are namespaced properly,
func keyWithPrefix(keyPrefix string, key string) string {
	return keyPrefix + ":" + key
}
