package auth

import (
	"context"
	"net/http"
)

// CustomerIDParam / CustomerIDHeader are how an unauthenticated caller supplies
// its identity when Cognito auth is disabled.
const (
	CustomerIDParam  = "customerId"
	CustomerIDHeader = "X-Customer-Id"
)

type customerCtxKey struct{}

func withCustomerID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, customerCtxKey{}, id)
}

func CustomerIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(customerCtxKey{}).(string)
	return id, ok
}

func DevCustomerMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.URL.Query().Get(CustomerIDParam)
			if id == "" {
				id = r.Header.Get(CustomerIDHeader)
			}
			next.ServeHTTP(w, r.WithContext(withCustomerID(r.Context(), id)))
		})
	}
}
