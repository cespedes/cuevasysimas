package assertion

import (
	"context"
	"crypto/ed25519"
	"errors"
	"sync"
	"time"

	"github.com/cespedes/cuevasysimas/internal/accounts"
	"github.com/cespedes/cuevasysimas/internal/auth"
)

// KeyStore says which public key an issuer signs with. accounts.Service is the real one.
type KeyStore interface {
	// IssuerKey is the key of the issuer, or accounts.ErrUnknownIssuer if there is no such client or it has none.
	IssuerKey(ctx context.Context, issuer string) (ed25519.PublicKey, error)
}

// keyTTL is how long a key is kept before it is asked for again, so that a key that changes holds in a
// minute and a request does not cost a query.
const keyTTL = time.Minute

// Resolver checks the assertions that arrive as bearer tokens: it is the auth.SessionResolver of the server.
type Resolver struct {
	keys KeyStore
	now  func() time.Time

	mu    sync.Mutex
	cache map[string]cachedKey
}

type cachedKey struct {
	key     ed25519.PublicKey
	fetched time.Time
}

func NewResolver(keys KeyStore) *Resolver {
	return &Resolver{keys: keys, now: time.Now, cache: map[string]cachedKey{}}
}

func (r *Resolver) key(ctx context.Context, issuer string) (ed25519.PublicKey, error) {
	r.mu.Lock()
	c, ok := r.cache[issuer]
	r.mu.Unlock()
	if ok && r.now().Sub(c.fetched) < keyTTL {
		return c.key, nil
	}
	key, err := r.keys.IssuerKey(ctx, issuer)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.cache[issuer] = cachedKey{key: key, fetched: r.now()}
	r.mu.Unlock()
	return key, nil
}

// Authenticate satisfies auth.SessionResolver. Whatever is wrong with the token (it is not an assertion, the
// issuer is not known, the signature does not match, it has expired) is the same answer to the caller,
// auth.ErrInvalidToken: only a database that fails is told apart.
func (r *Resolver) Authenticate(ctx context.Context, rawToken, _, _ string) (auth.AuthenticatedUser, error) {
	issuer, err := Issuer(rawToken)
	if err != nil {
		return auth.AuthenticatedUser{}, auth.ErrInvalidToken
	}
	key, err := r.key(ctx, issuer)
	if errors.Is(err, accounts.ErrUnknownIssuer) {
		return auth.AuthenticatedUser{}, auth.ErrInvalidToken
	}
	if err != nil {
		return auth.AuthenticatedUser{}, err
	}
	claims, err := Verify(rawToken, key, r.now())
	if err != nil {
		return auth.AuthenticatedUser{}, auth.ErrInvalidToken
	}
	return auth.AuthenticatedUser{
		Issuer: claims.Issuer, Subject: claims.Subject, Name: claims.Name, Groups: claims.Groups, Roles: claims.Roles,
		ExpiresAt: time.Unix(claims.ExpiresAt, 0),
	}, nil
}
