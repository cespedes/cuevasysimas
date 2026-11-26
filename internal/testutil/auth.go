package testutil

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/cespedes/cuevasysimas/internal/access"
	"github.com/cespedes/cuevasysimas/internal/accounts"
	"github.com/cespedes/cuevasysimas/internal/auth"
)

// FakeResolver is an in-memory auth.SessionResolver, so that the tests of the handlers can make a session for
// anybody without signing an assertion (internal/assertion has its own tests of the real one).
type FakeResolver map[string]auth.AuthenticatedUser

func (f FakeResolver) Authenticate(_ context.Context, rawToken, _, _ string) (auth.AuthenticatedUser, error) {
	user, ok := f[rawToken]
	if !ok {
		return auth.AuthenticatedUser{}, auth.ErrInvalidToken
	}
	return user, nil
}

// NewTestAuthenticator returns an authenticator backed by a FakeResolver.
func NewTestAuthenticator() (*auth.Authenticator, FakeResolver) {
	resolver := FakeResolver{}
	return auth.NewAuthenticator(resolver), resolver
}

// TokenFor registers and returns a fake bearer token for the person with that id (see NewPerson) with the given
// roles. The person is in the club's group and vouched for by the intranet, as the clients that the tests
// stand for are.
func TokenFor(resolver FakeResolver, id int64, roleNames ...string) string {
	token := fmt.Sprintf("test-token-%d-%s", id, strings.Join(roleNames, ","))
	subject, name := identityOf(id)
	resolver[token] = auth.AuthenticatedUser{
		Issuer: access.IntranetIssuer, Subject: subject, Name: name, Groups: []string{access.ClubGroup}, Roles: roleNames,
	}
	return token
}

// TokenWithoutGroup is TokenFor for an assertion about somebody who belongs to no group: the catalogue's API
// has nothing to say to them, whatever roles the issuer gives them.
func TokenWithoutGroup(resolver FakeResolver, id int64, roleNames ...string) string {
	token := fmt.Sprintf("test-token-nogroup-%d-%s", id, strings.Join(roleNames, ","))
	subject, name := identityOf(id)
	resolver[token] = auth.AuthenticatedUser{Issuer: access.IntranetIssuer, Subject: subject, Name: name, Roles: roleNames}
	return token
}

// Identified is somebody who has a subject and a name.
type Identified interface {
	Identity() (subject, name string)
}

// Person is somebody who acts on the catalogue: a subject and a name, with no row in any table.
// TokenFor(resolver, person.ID, roles...) makes a session for them.
type Person struct {
	ID      int64
	Subject string
	Login   string
}

// Identity says who they are, for the catalogue.
func (p Person) Identity() (subject, name string) { return p.Subject, p.Login }

var (
	personCounter atomic.Int64
	people        sync.Map
)

type personIdentity struct{ subject, name string }

// NewPerson is a person nobody has heard of. Its ID is made up, and only TokenFor reads it.
func NewPerson(t *testing.T) Person {
	t.Helper()
	n := personCounter.Add(1)
	p := Person{ID: n, Subject: fmt.Sprintf("00000000-0000-4000-8000-%012d", n), Login: fmt.Sprintf("test-person-%d", n)}
	people.Store(p.ID, personIdentity{subject: p.Subject, name: p.Login})
	return p
}

func identityOf(id int64) (subject, name string) {
	if p, ok := people.Load(id); ok {
		return p.(personIdentity).subject, p.(personIdentity).name
	}
	return fmt.Sprintf("00000000-0000-0000-0000-%012d", id), fmt.Sprintf("test-person-%d", id)
}

// CatalogueAccount is the account of the catalogue of a person, made if they have none: what the catalogue
// keeps of who made something, for a test that puts rows in the catalogue by hand.
func CatalogueAccount(t *testing.T, tx pgx.Tx, who Identified) int64 {
	t.Helper()
	subject, name := who.Identity()
	account, err := accounts.NewService(accounts.NewRepository(tx)).Ensure(context.Background(), access.Viewer{
		Issuer: access.IntranetIssuer, Subject: subject, Name: name,
	})
	if err != nil {
		t.Fatalf("the catalogue account of %s: %v", name, err)
	}
	return account.ID
}
