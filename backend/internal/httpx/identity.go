package httpx

import "context"

type userIDKey struct{}

// WithUserID carries the account verified by the authentication guard.
func WithUserID(ctx context.Context, id uint64) context.Context {
	return context.WithValue(ctx, userIDKey{}, id)
}

func UserID(ctx context.Context) uint64 {
	id, _ := ctx.Value(userIDKey{}).(uint64)
	return id
}
