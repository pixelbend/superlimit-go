package zlechovault

import (
	"context"
	"fmt"
	"github.com/driftdev/zenlimiter"
	"github.com/echovault/echovault/echovault"
	"math"
	"strconv"
	"time"
)

const keyPrefix = "rate:"

type EchoVaultClient interface {
	Set(key string, value string, options echovault.SetOptions) (string, bool, error)
	Get(key string) (string, error)
	Del(keys ...string) (int, error)
}

var _ zenlimiter.LimiterProvider = (*Backend)(nil)

type Backend struct {
	client EchoVaultClient
}

func NewBackend(client EchoVaultClient) *Backend {
	return &Backend{client: client}
}

func (b *Backend) Allow(ctx context.Context, key string, limit zenlimiter.Limit) (*zenlimiter.Result, error) {
	return b.AllowN(ctx, key, limit, 1)
}

func (b *Backend) AllowN(ctx context.Context, key string, limit zenlimiter.Limit, n int) (*zenlimiter.Result, error) {
	rateLimitKey := keyPrefix + key
	now := float64(time.Now().UnixNano()) / 1e9
	emissionInterval := limit.Period.Seconds() / float64(limit.Rate)
	burstOffset := emissionInterval * float64(limit.Burst)
	increment := emissionInterval * float64(n)

	tat, err := b.getTat(ctx, rateLimitKey, now)
	if err != nil {
		return nil, err
	}

	tat = math.Max(tat, now)
	newTat := tat + increment
	allowAt := newTat - burstOffset
	diff := now - allowAt
	remaining := diff / emissionInterval

	if remaining < 0 {
		resetAfter := tat - now
		retryAfter := -diff
		return &zenlimiter.Result{
			Limit:      limit,
			Allowed:    0,
			Remaining:  0,
			RetryAfter: dur(retryAfter),
			ResetAfter: dur(resetAfter),
		}, nil
	}

	resetAfter := newTat - now
	if resetAfter > 0 {
		_, _, err = b.client.Set(rateLimitKey, fmt.Sprintf("%f", newTat), echovault.SetOptions{
			EX: int((time.Duration(math.Ceil(resetAfter)) * time.Second).Seconds()),
		})
		if err != nil {
			return nil, err
		}
	}
	retryAfter := -1.0

	return &zenlimiter.Result{
		Limit:      limit,
		Allowed:    n,
		Remaining:  int(remaining),
		RetryAfter: dur(retryAfter),
		ResetAfter: dur(resetAfter),
	}, nil
}

func (b *Backend) AllowAtMost(ctx context.Context, key string, limit zenlimiter.Limit, n int) (*zenlimiter.Result, error) {
	rateLimitKey := keyPrefix + key
	now := float64(time.Now().UnixNano()) / 1e9
	emissionInterval := limit.Period.Seconds() / float64(limit.Rate)
	burstOffset := emissionInterval * float64(limit.Burst)

	tat, err := b.getTat(ctx, rateLimitKey, now)
	if err != nil {
		return nil, err
	}

	tat = math.Max(tat, now)
	diff := now - (tat - burstOffset)
	remaining := diff / emissionInterval

	if remaining < 1 {
		resetAfter := tat - now
		retryAfter := emissionInterval - diff
		return &zenlimiter.Result{
			Limit:      limit,
			Allowed:    0,
			Remaining:  0,
			RetryAfter: dur(retryAfter),
			ResetAfter: dur(resetAfter),
		}, nil
	}

	if remaining < float64(n) {
		n = int(remaining)
		remaining = 0
	} else {
		remaining -= float64(n)
	}

	increment := emissionInterval * float64(n)
	newTat := tat + increment
	resetAfter := newTat - now

	if resetAfter > 0 {
		_, _, err = b.client.Set(rateLimitKey, fmt.Sprintf("%f", newTat), echovault.SetOptions{
			EX: int((time.Duration(math.Ceil(resetAfter)) * time.Second).Seconds()),
		})
		if err != nil {
			return nil, err
		}
	}

	return &zenlimiter.Result{
		Limit:      limit,
		Allowed:    n,
		Remaining:  int(remaining),
		RetryAfter: dur(-1),
		ResetAfter: dur(resetAfter),
	}, nil
}

func (b *Backend) Reset(ctx context.Context, key string) error {
	_, err := b.client.Del(keyPrefix + key)
	if err != nil {
		return err
	}

	return nil
}

func (b *Backend) getTat(ctx context.Context, key string, now float64) (float64, error) {
	tatStr, err := b.client.Get(key)
	if tatStr == "" {
		return now, nil
	} else if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(tatStr, 64)
}

func dur(f float64) time.Duration {
	if f == -1 {
		return -1
	}
	return time.Duration(f * float64(time.Second))
}
