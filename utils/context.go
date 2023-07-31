package utils

import "context"

func WithContext(ctx context.Context, f func(ctx context.Context) error) func() error {
	return func() error {
		return f(ctx)
	}
}
