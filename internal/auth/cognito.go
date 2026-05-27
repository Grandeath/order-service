package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

const (
	cognitoIssuerFormat = "https://cognito-idp.%s.amazonaws.com/%s"
	jwksPath            = "/.well-known/jwks.json"

	jwksMinRefreshInterval = 15 * time.Minute
	jwksMaxRefreshInterval = 24 * time.Hour
)

type CognitoConfig struct {
	Region          string
	UserPoolID      string
	AppClientID     string   // when set, verifies aud (id token) or client_id (access token)
	AllowedTokenUse []string // default: {"access", "id"}
}

type CognitoVerifier struct {
	issuer          string
	keySet          jwk.Set
	appClientID     string
	allowedTokenUse map[string]struct{}
}

func NewCognitoVerifier(ctx context.Context, cfg CognitoConfig) (*CognitoVerifier, error) {
	if cfg.Region == "" || cfg.UserPoolID == "" {
		return nil, errors.New("cognito region and user pool id are required")
	}
	issuer := fmt.Sprintf(cognitoIssuerFormat, cfg.Region, cfg.UserPoolID)
	jwksURL := issuer + jwksPath

	cache, err := jwk.NewCache(ctx, httprc.NewClient())
	if err != nil {
		return nil, fmt.Errorf("init jwks cache: %w", err)
	}
	if err := cache.Register(ctx, jwksURL,
		jwk.WithMinInterval(jwksMinRefreshInterval),
		jwk.WithMaxInterval(jwksMaxRefreshInterval),
	); err != nil {
		return nil, fmt.Errorf("register jwks: %w", err)
	}

	cached, err := cache.CachedSet(jwksURL)
	if err != nil {
		return nil, fmt.Errorf("cached set: %w", err)
	}

	allowed := map[string]struct{}{"access": {}, "id": {}}
	if len(cfg.AllowedTokenUse) > 0 {
		allowed = make(map[string]struct{}, len(cfg.AllowedTokenUse))
		for _, u := range cfg.AllowedTokenUse {
			allowed[u] = struct{}{}
		}
	}

	return &CognitoVerifier{
		issuer:          issuer,
		keySet:          cached,
		appClientID:     cfg.AppClientID,
		allowedTokenUse: allowed,
	}, nil
}

func (v *CognitoVerifier) Verify(_ context.Context, raw string) (jwt.Token, error) {
	tok, err := jwt.Parse(
		[]byte(raw),
		jwt.WithKeySet(v.keySet),
		jwt.WithValidate(true),
		jwt.WithIssuer(v.issuer),
		jwt.WithAcceptableSkew(30*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("token verify: %w", err)
	}

	var tokenUse string
	if err := tok.Get("token_use", &tokenUse); err != nil {
		return nil, fmt.Errorf("token_use claim missing: %w", err)
	}
	if _, ok := v.allowedTokenUse[tokenUse]; !ok {
		return nil, fmt.Errorf("token_use %q not allowed", tokenUse)
	}

	if v.appClientID != "" {
		clientID, err := extractClientID(tok, tokenUse)
		if err != nil {
			return nil, err
		}
		if clientID != v.appClientID {
			return nil, errors.New("token client_id/aud does not match configured app client id")
		}
	}

	return tok, nil
}

func extractClientID(tok jwt.Token, tokenUse string) (string, error) {
	switch tokenUse {
	case "id":
		aud, ok := tok.Audience()
		if !ok || len(aud) == 0 {
			return "", errors.New("id token missing aud")
		}
		return aud[0], nil
	case "access":
		var clientID string
		if err := tok.Get("client_id", &clientID); err != nil {
			return "", fmt.Errorf("access token missing client_id: %w", err)
		}
		return clientID, nil
	default:
		return "", fmt.Errorf("unsupported token_use %q", tokenUse)
	}
}
