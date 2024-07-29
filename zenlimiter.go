package zenlimiter

import "context"

type LimiterProvider interface {
	Allow(ctx context.Context, key string, limit Limit) (*Result, error)
	AllowN(ctx context.Context, key string, limit Limit, n int) (*Result, error)
	AllowAtMost(ctx context.Context, key string, limit Limit, n int) (*Result, error)
	Reset(ctx context.Context, key string) error
}

type Limiter struct {
	limiter LimiterProvider
}

func NewLimiter(limiter LimiterProvider) *Limiter {
	return &Limiter{
		limiter: limiter,
	}
}

func (l *Limiter) Allow(ctx context.Context, key string, limit Limit) (*Result, error) {
	return l.limiter.Allow(ctx, key, limit)
}

func (l *Limiter) AllowN(ctx context.Context, key string, limit Limit, n int) (*Result, error) {
	return l.limiter.AllowN(ctx, key, limit, n)
}

func (l *Limiter) AllowAtMost(ctx context.Context, key string, limit Limit, n int) (*Result, error) {
	return l.limiter.AllowAtMost(ctx, key, limit, n)
}

func (l *Limiter) Reset(ctx context.Context, key string) error {
	return l.limiter.Reset(ctx, key)
}
