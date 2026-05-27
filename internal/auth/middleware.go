package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/Grandeath/order-service/internal/utils"
)

// Verifier is the contract the middleware depends on. CognitoVerifier is the
// production impl; tests can substitute a fake.
type Verifier interface {
	Verify(ctx context.Context, raw string) (jwt.Token, error)
}

type ctxKey string

const tokenCtxKey ctxKey = "jwt-token"

// Middleware enforces Bearer auth. On success the parsed token is attached to
// the request context; handlers retrieve it via TokenFromContext.
func Middleware(v Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, err := bearerToken(r)
			if err != nil {
				writeUnauthorized(w, err)
				return
			}
			tok, err := v.Verify(r.Context(), raw)
			if err != nil {
				writeUnauthorized(w, err)
				return
			}
			ctx := context.WithValue(r.Context(), tokenCtxKey, tok)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func TokenFromContext(ctx context.Context) (jwt.Token, bool) {
	t, ok := ctx.Value(tokenCtxKey).(jwt.Token)
	return t, ok
}

// SubjectFromContext is a convenience for handlers that just need the user id.
func SubjectFromContext(ctx context.Context) (string, bool) {
	tok, ok := TokenFromContext(ctx)
	if !ok {
		return "", false
	}
	sub, ok := tok.Subject()
	return sub, ok
}

func bearerToken(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", errors.New("missing Authorization header")
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid Authorization scheme; expected Bearer")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("empty bearer token")
	}
	return token, nil
}

func writeUnauthorized(w http.ResponseWriter, err error) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="order-service"`)
	utils.WriteJSON(w, http.StatusUnauthorized, map[string]string{
		"error": err.Error(),
		"code":  "unauthorized",
	})
}
