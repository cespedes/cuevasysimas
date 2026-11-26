package assertion_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/cespedes/cuevasysimas/internal/assertion"
)

var now = time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)

func keys(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func claims() assertion.Claims {
	return assertion.Claims{
		Issuer: "club", Subject: "7f2c", Name: "ana", Groups: []string{"katiuskas"}, Roles: []string{"admin"},
		IssuedAt: now.Unix(), ExpiresAt: now.Add(5 * time.Minute).Unix(),
	}
}

func TestSignAndVerify(t *testing.T) {
	pub, priv := keys(t)
	token, err := assertion.Sign(priv, claims())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, "v1.") || strings.Count(token, ".") != 2 {
		t.Errorf("a token is v1.<payload>.<signature>: %q", token)
	}
	got, err := assertion.Verify(token, pub, now)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.Issuer != "club" || got.Subject != "7f2c" || got.Name != "ana" || len(got.Groups) != 1 || got.Roles[0] != "admin" {
		t.Errorf("what it says: %+v", got)
	}
	if issuer, err := assertion.Issuer(token); err != nil || issuer != "club" {
		t.Errorf("the issuer is readable before the signature is checked: %q %v", issuer, err)
	}
}

// What is refused, and why: the signature has to be the issuer's over exactly these bytes, and the time has
// to be right.
func TestVerifyRefuses(t *testing.T) {
	pub, priv := keys(t)
	otherPub, otherPriv := keys(t)
	good, _ := assertion.Sign(priv, claims())

	parts := strings.Split(good, ".")
	tamper := func(c assertion.Claims) string {
		// A payload that says something else, with the signature of the original: it must not hold.
		forged, _ := assertion.Sign(otherPriv, c)
		fp := strings.Split(forged, ".")
		return parts[0] + "." + fp[1] + "." + parts[2]
	}
	admin := claims()
	admin.Roles = []string{"admin", "root"}

	long := claims()
	long.ExpiresAt = now.Add(assertion.MaxLifetime + time.Second).Unix()
	reversed := claims()
	reversed.ExpiresAt = now.Add(-time.Minute).Unix()
	noName := claims()
	noName.Name = " "
	noSubject := claims()
	noSubject.Subject = ""
	noIssuer := claims()
	noIssuer.Issuer = ""

	signed := func(c assertion.Claims) string { t, _ := assertion.Sign(priv, c); return t }
	cases := []struct {
		name  string
		token string
		key   ed25519.PublicKey
		at    time.Time
		want  error
	}{
		{"signed by another key", good, otherPub, now, assertion.ErrBadSignature},
		{"a payload changed afterwards", tamper(admin), pub, now, assertion.ErrBadSignature},
		{"no key", good, nil, now, assertion.ErrBadSignature},
		{"not an assertion", "hola", pub, now, assertion.ErrMalformed},
		{"another version", "v2." + parts[1] + "." + parts[2], pub, now, assertion.ErrMalformed},
		{"a payload that is not base64", "v1.@@@." + parts[2], pub, now, assertion.ErrMalformed},
		{"a signature of the wrong length", "v1." + parts[1] + ".AAAA", pub, now, assertion.ErrMalformed},
		{"expired", good, pub, now.Add(5*time.Minute + time.Minute), assertion.ErrExpired},
		{"not yet valid", good, pub, now.Add(-time.Minute), assertion.ErrExpired},
		{"it lasts too long", signed(long), pub, now, assertion.ErrExpired},
		{"it ends before it begins", signed(reversed), pub, now, assertion.ErrExpired},
		{"no name", signed(noName), pub, now, assertion.ErrMalformed},
		{"no subject", signed(noSubject), pub, now, assertion.ErrMalformed},
		{"no issuer", signed(noIssuer), pub, now, assertion.ErrMalformed},
	}
	for _, c := range cases {
		if _, err := assertion.Verify(c.token, c.key, c.at); err != c.want {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
	// The clocks of the two sides may differ a little.
	if _, err := assertion.Verify(good, pub, now.Add(-10*time.Second)); err != nil {
		t.Errorf("a clock a few seconds behind: %v", err)
	}
	if _, err := assertion.Verify(good, pub, now.Add(5*time.Minute+10*time.Second)); err != nil {
		t.Errorf("a clock a few seconds ahead: %v", err)
	}
}
