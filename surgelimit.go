package surgelimit

import "time"

const defaultKeyPrefix = "RATE_LIMIT"

// Options holds the configuration settings for the LeakyBucketLimiter.
type Options struct {
	// KeyPrefix is the prefix added to all keys used by the limiter.
	// This helps to distinguish rate-limiting keys from other keys.
	// By default, it is set to "RATE_LIMIT", but it can be customized to avoid key collisions
	KeyPrefix string
}

// DefaultOptions returns an Options struct populated with the default settings
// for the LeakyBucketLimiter. Currently, this includes setting the `KeyPrefix` to "RATE_LIMIT".
//
// Defaults:
//
//   - KeyPrefix: The prefix used for keys managed by the limiter. Default is set to "RATE_LIMIT".
func DefaultOptions() Options {
	return Options{
		KeyPrefix: defaultKeyPrefix,
	}
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
