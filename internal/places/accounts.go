package places

import (
	"context"

	"github.com/cespedes/cuevasysimas/internal/access"
	"github.com/cespedes/cuevasysimas/internal/accounts"
)

// Who did something is an account of the catalogue (internal/accounts), made the first time a person
// acts: the places keep its id and show its name, and never read how the intranet stores people.

// AccountOf is the account of the viewer, made if it is the first time they act. It is what a route that
// changes something asks for before it does.
func (s *Service) AccountOf(ctx context.Context, viewer access.Viewer) (accounts.Account, error) {
	return s.accounts.Ensure(ctx, viewer)
}

// HasAuthored says whether the person that the issuer knows by that subject made anything in the
// catalogue of places (users.AuthorGuard). One who never acted has no account, and made nothing.
func (s *Service) HasAuthored(ctx context.Context, issuer, subject string) (bool, error) {
	account, found, err := s.accounts.Of(ctx, issuer, subject)
	if err != nil || !found {
		return false, err
	}
	return s.repo.HasAuthored(ctx, account.ID)
}
