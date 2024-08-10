package zenlimit

import (
	"context"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
)

const keyPrefix = "RATE_LIMIT"

type Options struct {
	KeyPrefix string
}

func DefaultOptions() Options {
	return Options{
		KeyPrefix: keyPrefix,
	}
}

type Limiter struct {
	client  redis.UniversalClient
	Options Options
}

func NewLimiter(client redis.UniversalClient, options Options) *Limiter {
	return &Limiter{
		client:  client,
		Options: options,
	}
}

func (l *Limiter) Allow(ctx context.Context, key string, limit Limit) (*Result, error) {
	return l.AllowN(ctx, key, limit, 1)
}

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

func (l *Limiter) Reset(ctx context.Context, key string) error {
	return l.client.Del(ctx, keyWithPrefix(l.Options.KeyPrefix, key)).Err()
}

func dur(f float64) time.Duration {
	if f == -1 {
		return -1
	}
	return time.Duration(f * float64(time.Second))
}

func keyWithPrefix(keyPrefix string, key string) string {
	return keyPrefix + ":" + key
}
