package requestctx

import "context"

type requestIDKey struct{}

// WithRequestID returns a context carrying the identifier assigned to the
// inbound HTTP request.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

// RequestID returns the inbound HTTP request identifier, or an empty string
// when the context did not originate from the HTTP middleware.
func RequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}
