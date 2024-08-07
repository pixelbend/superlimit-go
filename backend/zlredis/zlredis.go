package zlredis

import (
	"context"
	"github.com/driftdev/zenlimit"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
)

const keyPrefix = "rate:"

type RedisClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
	EvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) *redis.Cmd
	ScriptExists(ctx context.Context, hashes ...string) *redis.BoolSliceCmd
	ScriptLoad(ctx context.Context, script string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	EvalRO(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd
	EvalShaRO(ctx context.Context, sha1 string, keys []string, args ...interface{}) *redis.Cmd
}

var _ zenlimit.LimiterProvider = (*Backend)(nil)

type Backend struct {
	client RedisClient
}

func NewBackend(client RedisClient) *Backend {
	return &Backend{client: client}
}

func (b *Backend) Allow(ctx context.Context, key string, limit zenlimit.Limit) (*zenlimit.Result, error) {
	return b.AllowN(ctx, key, limit, 1)
}

func (b *Backend) AllowN(
	ctx context.Context,
	key string,
	limit zenlimit.Limit,
	n int,
) (*zenlimit.Result, error) {
	values := []interface{}{limit.Burst, limit.Rate, limit.Period.Seconds(), n}
	v, err := allowN.Run(ctx, b.client, []string{keyPrefix + key}, values...).Result()
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

	res := &zenlimit.Result{
		Limit:      limit,
		Allowed:    int(values[0].(int64)),
		Remaining:  int(values[1].(int64)),
		RetryAfter: dur(retryAfter),
		ResetAfter: dur(resetAfter),
	}
	return res, nil
}

func (b *Backend) AllowAtMost(
	ctx context.Context,
	key string,
	limit zenlimit.Limit,
	n int,
) (*zenlimit.Result, error) {
	values := []interface{}{limit.Burst, limit.Rate, limit.Period.Seconds(), n}
	v, err := allowAtMost.Run(ctx, b.client, []string{keyPrefix + key}, values...).Result()
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

	res := &zenlimit.Result{
		Limit:      limit,
		Allowed:    int(values[0].(int64)),
		Remaining:  int(values[1].(int64)),
		RetryAfter: dur(retryAfter),
		ResetAfter: dur(resetAfter),
	}
	return res, nil
}

func (b *Backend) Reset(ctx context.Context, key string) error {
	return b.client.Del(ctx, keyPrefix+key).Err()
}

func dur(f float64) time.Duration {
	if f == -1 {
		return -1
	}
	return time.Duration(f * float64(time.Second))
}
