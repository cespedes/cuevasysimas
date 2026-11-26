// Package assertion is how a client of the catalogue says who is acting: a short-lived statement about a
// person that the client signs with Ed25519 (crypto/ed25519, of the standard library) and the catalogue
// checks with the public key it holds for that client. Nothing is looked up and nothing is kept: the
// catalogue never calls the client to know a person, and a request carries the whole proof.
//
// A token is `v1.<payload>.<signature>`, the payload the JSON of Claims and the signature of the bytes
// `v1.<payload>`, both in base64url with no padding:
//
//	iss     the issuer, the name of the client as the catalogue knows it
//	sub     who the person is for the issuer: opaque, and the same for the same person every time
//	name    how the issuer calls them (what is shown as the author of a change)
//	groups  the groups they belong to
//	roles   what they are in them
//	iat     when it was made, in seconds of Unix time
//	exp     when it stops holding: at most MaxLifetime after iat
//
// The same few lines (Sign, with the claims) are what a client copies to make its assertions.
package assertion

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// MaxLifetime is the longest an assertion may hold, which the catalogue enforces: a client that makes them
// longer is refused, so that one that leaks does not hold for long.
const MaxLifetime = 10 * time.Minute

// clockSkew is how far the clocks of the two sides may differ.
const clockSkew = 30 * time.Second

const version = "v1"

var (
	// ErrMalformed is a token that is not an assertion.
	ErrMalformed = errors.New("malformed assertion")
	// ErrBadSignature is an assertion that was not signed with the key of its issuer.
	ErrBadSignature = errors.New("bad signature")
	// ErrExpired is an assertion that does not hold now: it has expired, was made in the future, or lasts
	// longer than MaxLifetime.
	ErrExpired = errors.New("assertion expired or not yet valid")
)

// Claims is what an assertion says.
type Claims struct {
	Issuer    string   `json:"iss"`
	Subject   string   `json:"sub"`
	Name      string   `json:"name"`
	Groups    []string `json:"groups"`
	Roles     []string `json:"roles"`
	IssuedAt  int64    `json:"iat"`
	ExpiresAt int64    `json:"exp"`
}

var b64 = base64.RawURLEncoding

// Sign makes the assertion of the claims with the private key of the issuer.
func Sign(key ed25519.PrivateKey, c Claims) (string, error) {
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	signed := version + "." + b64.EncodeToString(payload)
	return signed + "." + b64.EncodeToString(ed25519.Sign(key, []byte(signed))), nil
}

// Issuer is the issuer an assertion says it is from, without having checked anything: it is only what tells
// which key to check it with.
func Issuer(token string) (string, error) {
	c, _, _, err := parse(token)
	if err != nil {
		return "", err
	}
	return c.Issuer, nil
}

func parse(token string) (c Claims, signed string, sig []byte, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != version {
		return Claims{}, "", nil, ErrMalformed
	}
	payload, err := b64.DecodeString(parts[1])
	if err != nil {
		return Claims{}, "", nil, ErrMalformed
	}
	if sig, err = b64.DecodeString(parts[2]); err != nil || len(sig) != ed25519.SignatureSize {
		return Claims{}, "", nil, ErrMalformed
	}
	if err := json.Unmarshal(payload, &c); err != nil {
		return Claims{}, "", nil, ErrMalformed
	}
	return c, parts[0] + "." + parts[1], sig, nil
}

// Verify checks the signature of an assertion with the public key of its issuer and that it holds at `now`,
// and answers what it says. An assertion with no issuer, subject or name says nothing, and is refused.
func Verify(token string, key ed25519.PublicKey, now time.Time) (Claims, error) {
	c, signed, sig, err := parse(token)
	if err != nil {
		return Claims{}, err
	}
	if len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, []byte(signed), sig) {
		return Claims{}, ErrBadSignature
	}
	if strings.TrimSpace(c.Issuer) == "" || strings.TrimSpace(c.Subject) == "" || strings.TrimSpace(c.Name) == "" {
		return Claims{}, ErrMalformed
	}
	issued, expires := time.Unix(c.IssuedAt, 0), time.Unix(c.ExpiresAt, 0)
	if expires.Sub(issued) > MaxLifetime || expires.Before(issued) ||
		now.Before(issued.Add(-clockSkew)) || !now.Before(expires.Add(clockSkew)) {
		return Claims{}, ErrExpired
	}
	return c, nil
}
