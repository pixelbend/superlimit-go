package leakybucket

import "time"

// fmtDur returns a string representation of the time.Duration for common periods.
// Example: "1s" for one second, "1m" for one minute, "1h" for one hour. For other durations, it returns the default string format.
func fmtDur(d time.Duration) string {
	switch d {
	case time.Second:
		return "s"
	case time.Minute:
		return "m"
	case time.Hour:
		return "h"
	}
	return d.String()
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
