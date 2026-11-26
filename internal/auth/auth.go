// Package auth is who is asking, for the API that the clients of the catalogue use. A client (the
// intranet of the club is the first) proves who is acting with an assertion that it signs
// (internal/assertion): the issuer, the person, their name, the groups they belong to and the roles they
// have there. The catalogue never calls a client to check a person, and never keeps a session.
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/cespedes/cuevasysimas/internal/access"
	"github.com/cespedes/cuevasysimas/internal/httpapi"
)

type contextKey string

const userContextKey contextKey = "auth.user"

// ErrInvalidToken is a token that does not hold: it is not an assertion, its signature is wrong, or it has
// expired.
var ErrInvalidToken = errors.New("invalid or expired token")

// AuthenticatedUser is a person that an issuer vouches for, as the catalogue knows them for one request.
type AuthenticatedUser struct {
	// Issuer is who vouches for them and Subject who they are for that issuer, which the catalogue never
	// reads anything from. Name is how the issuer calls them: what is shown as the author of a change.
	Issuer  string
	Subject string
	Name    string
	// Groups are the groups they belong to and Roles what they are in them (admin, board…), as the issuer
	// says them.
	Groups []string
	Roles  []string
	// ExpiresAt is when the assertion stops being true.
	ExpiresAt time.Time
}

// HasAnyRole reports whether the user holds at least one of the given roles.
func (u AuthenticatedUser) HasAnyRole(names ...string) bool {
	for _, want := range names {
		for _, have := range u.Roles {
			if have == want {
				return true
			}
		}
	}
	return false
}

// SessionResolver turns a bearer token into the user it vouches for: internal/assertion is the real one,
// which checks a signature; the tests of the handlers use a fake that keeps tokens in memory.
type SessionResolver interface {
	Authenticate(ctx context.Context, rawToken, ip, userAgent string) (AuthenticatedUser, error)
}

type Authenticator struct {
	resolver SessionResolver
}

func NewAuthenticator(resolver SessionResolver) *Authenticator {
	return &Authenticator{resolver: resolver}
}

func bearerToken(r *http.Request) string {
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return strings.TrimSpace(token)
}

func (a *Authenticator) authenticate(r *http.Request, token string) (AuthenticatedUser, error) {
	user, err := a.resolver.Authenticate(r.Context(), token, httpapi.ClientIP(r), r.UserAgent())
	if err != nil {
		return AuthenticatedUser{}, ErrInvalidToken
	}
	httpapi.SetRequestSubject(r.Context(), user.Issuer+":"+user.Subject)
	return user, nil
}

// RequireAuth rejects the request unless it carries an assertion that holds.
func (a *Authenticator) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}
		user, err := a.authenticate(r, token)
		if err != nil {
			httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

// WithViewer is the authentication of a route that answers anybody, and answers more to some: with a bearer
// token the person is resolved as RequireAuth does and the handler finds them (UserFromContext,
// ViewerFromContext); with none the request goes on as anonymous. A token that does not hold is 401, not an
// anonymous request, so that a client never gets the public answer by mistake when its assertion has
// expired. It asks for no group: what each viewer may see is for the handler to ask of the rules
// (access.Rules.Allows), and it must, since nothing here refuses anybody.
func (a *Authenticator) WithViewer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		user, err := a.authenticate(r, token)
		if err != nil {
			httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

// RequireRole rejects the request unless the authenticated user holds at least one of the given roles. It
// must run after RequireAuth.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok || !user.HasAnyRole(roles...) {
				httpapi.WriteError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requireGroup rejects the request unless the user belongs to a group: an assertion about somebody who is in
// none is nobody the catalogue's API has anything to say to.
func requireGroup(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok || len(user.Groups) == 0 {
			httpapi.WriteError(w, http.StatusForbidden, "forbidden", "insufficient permissions")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func UserFromContext(ctx context.Context) (AuthenticatedUser, bool) {
	user, ok := ctx.Value(userContextKey).(AuthenticatedUser)
	return user, ok
}

// Protect wraps a handler with authentication and authorization: the assertion has to hold and be about
// somebody who belongs to a group, and, when roles are given too, who holds at least one of them.
func (a *Authenticator) Protect(next http.HandlerFunc, roles ...string) http.Handler {
	var handler http.Handler = next
	if len(roles) > 0 {
		handler = RequireRole(roles...)(handler)
	}
	return a.RequireAuth(requireGroup(handler))
}

// ViewerOf is the viewer an authenticated user is for the rules of the catalogue.
func ViewerOf(user AuthenticatedUser) access.Viewer {
	return access.Viewer{
		Issuer: user.Issuer, Subject: user.Subject, Name: user.Name, Groups: user.Groups, ExpiresAt: user.ExpiresAt,
	}
}

// ViewerFromContext is who is asking, for the question of what they may see: the person the assertion
// vouches for if the request has one, and anonymous otherwise.
func ViewerFromContext(ctx context.Context) access.Viewer {
	if user, ok := UserFromContext(ctx); ok {
		return ViewerOf(user)
	}
	return access.Anonymous()
}
