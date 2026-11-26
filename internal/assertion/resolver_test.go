package assertion_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cespedes/cuevasysimas/internal/accounts"
	"github.com/cespedes/cuevasysimas/internal/assertion"
	"github.com/cespedes/cuevasysimas/internal/auth"
	"github.com/cespedes/cuevasysimas/internal/testutil"
)

// The real resolver, against the real table of issuers: what a client says holds if it is signed with the
// key registered for it, and in every other case the answer is the same.
func TestResolverChecksTheKeyOfTheIssuer(t *testing.T) {
	tx := testutil.NewTx(t)
	ctx := context.Background()
	svc := accounts.NewService(accounts.NewRepository(tx))
	pub, priv := keys(t)
	if err := svc.RegisterIssuer(ctx, "club", "Club", pub); err != nil {
		t.Fatal(err)
	}
	resolver := assertion.NewResolver(svc)
	c := claims()
	c.IssuedAt, c.ExpiresAt = time.Now().Unix(), time.Now().Add(5*time.Minute).Unix()
	token, _ := assertion.Sign(priv, c)

	user, err := resolver.Authenticate(ctx, token, "192.0.2.1", "test")
	if err != nil {
		t.Fatalf("a good assertion: %v", err)
	}
	if user.Issuer != "club" || user.Subject != "7f2c" || user.Name != "ana" || !user.HasAnyRole("admin") ||
		len(user.Groups) != 1 || user.ExpiresAt.IsZero() {
		t.Errorf("who it is: %+v", user)
	}

	// An issuer that is not registered, one registered with no key, one that signs with another key, and a
	// token that is nothing: all of them are refused the same way.
	_, otherPriv := keys(t)
	stranger := c
	stranger.Issuer = "stranger"
	strangerToken, _ := assertion.Sign(priv, stranger)
	wrongKey, _ := assertion.Sign(otherPriv, c)
	keyless := c
	keyless.Issuer = "keyless"
	keylessToken, _ := assertion.Sign(priv, keyless)
	if _, err := tx.Exec(ctx, `WITH a AS (INSERT INTO catalogue.accounts (kind, display_name) VALUES ('service', 'Sin clave') RETURNING id)
		INSERT INTO catalogue.issuers (issuer, account_id) SELECT 'keyless', id FROM a`); err != nil {
		t.Fatal(err)
	}
	for name, tok := range map[string]string{"unknown issuer": strangerToken, "no key": keylessToken, "another key": wrongKey, "nonsense": "hola"} {
		if _, err := resolver.Authenticate(ctx, tok, "", ""); !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("%s: %v, want ErrInvalidToken", name, err)
		}
	}
	// Expired.
	old := c
	old.IssuedAt, old.ExpiresAt = time.Now().Add(-time.Hour).Unix(), time.Now().Add(-55*time.Minute).Unix()
	oldToken, _ := assertion.Sign(priv, old)
	if _, err := resolver.Authenticate(ctx, oldToken, "", ""); !errors.Is(err, auth.ErrInvalidToken) {
		t.Errorf("expired: %v, want ErrInvalidToken", err)
	}
}

// A key that is changed holds from the next minute: until then the resolver may still have the old one.
func TestResolverKeepsAKeyOnlyForAWhile(t *testing.T) {
	tx := testutil.NewTx(t)
	ctx := context.Background()
	svc := accounts.NewService(accounts.NewRepository(tx))
	oldPub, oldPriv := keys(t)
	newPub, newPriv := keys(t)
	if err := svc.RegisterIssuer(ctx, "club", "Club", oldPub); err != nil {
		t.Fatal(err)
	}
	resolver := assertion.NewResolver(svc)
	c := claims()
	c.IssuedAt, c.ExpiresAt = time.Now().Unix(), time.Now().Add(5*time.Minute).Unix()
	oldToken, _ := assertion.Sign(oldPriv, c)
	newToken, _ := assertion.Sign(newPriv, c)
	if _, err := resolver.Authenticate(ctx, oldToken, "", ""); err != nil {
		t.Fatalf("the old key: %v", err)
	}
	if err := svc.RegisterIssuer(ctx, "club", "Club", newPub); err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.Authenticate(ctx, oldToken, "", ""); err != nil {
		t.Errorf("the old key still holds while it is kept: %v", err)
	}
	if _, err := resolver.Authenticate(ctx, newToken, "", ""); err == nil {
		t.Error("the new key is not known yet to a resolver that keeps the old one")
	}
	// A new resolver (or the same one a minute later) knows the new key and not the old one.
	fresh := assertion.NewResolver(svc)
	if _, err := fresh.Authenticate(ctx, newToken, "", ""); err != nil {
		t.Errorf("the new key: %v", err)
	}
	if _, err := fresh.Authenticate(ctx, oldToken, "", ""); err == nil {
		t.Error("the old key must not hold once it is changed")
	}
}
