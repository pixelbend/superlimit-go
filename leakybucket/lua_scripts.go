package leakybucket

import "github.com/redis/go-redis/v9"

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
