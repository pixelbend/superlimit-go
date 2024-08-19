package surgelimit

import (
	"context"
	"github.com/redis/go-redis/v9"
	"strconv"
)

// allowN is a Redis Lua script used for handling rate limiting based on the number of requests.
// It calculates how many of the requested `n` requests can be allowed given the current rate limit state.
//
// The script performs the following operations:
//   - Retrieves the current time and calculates time-based values for rate limiting.
//   - Checks the existing rate limit state stored in Redis.
//   - Determines the number of requests allowed based on the provided cost and rate limit parameters.
//   - Updates the rate limit state in Redis and returns information on allowed requests, remaining requests,
//     and the times to retry or reset the rate limit.
//
// Parameters used in the script:
//   - KEYS[1]: The key used for rate limiting in Redis.
//   - ARGV[1]: The burst size (maximum number of requests that can be handled in a burst).
//   - ARGV[2]: The rate (number of requests allowed per period).
//   - ARGV[3]: The period (duration in seconds for the rate limit).
//   - ARGV[4]: The cost (number of requests being attempted).
//
// Example usage in Go:
//
//	result, err := allowN.Run(ctx, redisClient, []string{"RATE_LIMIT:user_1234"}, burst, rate, period.Seconds(), n).Result()
//	// Handle result and error
var allowN = redis.NewScript(`
redis.replicate_commands()

local rate_limit_key = KEYS[1]
local burst = ARGV[1]
local rate = ARGV[2]
local period = ARGV[3]
local cost = tonumber(ARGV[4])

local emission_interval = period / rate
local increment = emission_interval * cost
local burst_offset = emission_interval * burst

local jan_1_2017 = 1483228800
local now = redis.call("TIME")
now = (now[1] - jan_1_2017) + (now[2] / 1000000)

local tat = redis.call("GET", rate_limit_key)

if not tat then
  tat = now
else
  tat = tonumber(tat)
end

tat = math.max(tat, now)

local new_tat = tat + increment
local allow_at = new_tat - burst_offset

local diff = now - allow_at
local remaining = diff / emission_interval

if remaining < 0 then
  local reset_after = tat - now
  local retry_after = diff * -1
  return {
    0, -- allowed
    0, -- remaining
    tostring(retry_after),
    tostring(reset_after),
  }
end

local reset_after = new_tat - now
if reset_after > 0 then
  redis.call("SET", rate_limit_key, new_tat, "EX", math.ceil(reset_after))
end
local retry_after = -1
return {cost, remaining, tostring(retry_after), tostring(reset_after)}
`)

// allowAtMost is a Redis Lua script used for handling rate limiting based on the maximum number of requests.
// It calculates the maximum number of requests that can be allowed, considering the burst capacity
// and rate limit state. The script ensures that no more than the allowed number of requests is processed.
//
// The script performs the following operations:
//   - Retrieves the current time and calculates time-based values for rate limiting.
//   - Checks the existing rate limit state stored in Redis.
//   - Determines the maximum number of requests that can be allowed based on the provided cost and rate limit parameters.
//   - Updates the rate limit state in Redis and returns information on allowed requests, remaining requests,
//     and the times to retry or reset the rate limit.
//
// Parameters used in the script:
//   - KEYS[1]: The key used for rate limiting in Redis.
//   - ARGV[1]: The burst size (maximum number of requests that can be handled in a burst).
//   - ARGV[2]: The rate (number of requests allowed per period).
//   - ARGV[3]: The period (duration in seconds for the rate limit).
//   - ARGV[4]: The cost (number of requests being attempted).
//
// Example usage in Go:
//
//	result, err := allowAtMost.Run(ctx, redisClient, []string{"RATE_LIMIT:user_1234"}, burst, rate, period.Seconds(), n).Result()
//	// Handle result and error
var allowAtMost = redis.NewScript(`
redis.replicate_commands()

local rate_limit_key = KEYS[1]
local burst = ARGV[1]
local rate = ARGV[2]
local period = ARGV[3]
local cost = tonumber(ARGV[4])

local emission_interval = period / rate
local burst_offset = emission_interval * burst

local jan_1_2017 = 1483228800
local now = redis.call("TIME")
now = (now[1] - jan_1_2017) + (now[2] / 1000000)

local tat = redis.call("GET", rate_limit_key)

if not tat then
  tat = now
else
  tat = tonumber(tat)
end

tat = math.max(tat, now)

local diff = now - (tat - burst_offset)
local remaining = diff / emission_interval

if remaining < 1 then
  local reset_after = tat - now
  local retry_after = emission_interval - diff
  return {
    0, -- allowed
    0, -- remaining
    tostring(retry_after),
    tostring(reset_after),
  }
end

if remaining < cost then
  cost = remaining
  remaining = 0
else
  remaining = remaining - cost
end

local increment = emission_interval * cost
local new_tat = tat + increment

local reset_after = new_tat - now
if reset_after > 0 then
  redis.call("SET", rate_limit_key, new_tat, "EX", math.ceil(reset_after))
end

return {
  cost,
  remaining,
  tostring(-1),
  tostring(reset_after),
}
`)

// LeakyBucketLimiter is the main struct that handles rate limiting. It uses a Redis client
// to store and retrieve rate-limiting information and operates based on the
// provided Options.
//
// Fields:
//   - client: A Redis client used to interact with the Redis or Redis Compatible database.
//   - Options: Configuration options for the LeakyBucketLimiter
type LeakyBucketLimiter struct {
	client  redis.UniversalClient
	Options Options
}

// NewLeakyBucketLimiter creates and returns a new LeakyBucketLimiter instance using the provided
// Redis client and options. The LeakyBucketLimiter is responsible for managing rate limits
// based on the given configuration.
//
// Parameters:
//   - client: A Redis UniversalClient used to interact with the Redis database.
//   - options: An Options struct that configures the LeakyBucketLimiter, including the key prefix.
//
// Returns:
//   - *LeakyBucketLimiter: A pointer to the newly created LeakyBucketLimiter instance.
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
func NewLeakyBucketLimiter(client redis.UniversalClient, options Options) *LeakyBucketLimiter {
	return &LeakyBucketLimiter{
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
func (l *LeakyBucketLimiter) Allow(ctx context.Context, key string, limit Limit) (*Result, error) {
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
func (l *LeakyBucketLimiter) AllowN(
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
func (l *LeakyBucketLimiter) AllowAtMost(
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
func (l *LeakyBucketLimiter) Reset(ctx context.Context, key string) error {
	return l.client.Del(ctx, keyWithPrefix(l.Options.KeyPrefix, key)).Err()
}
