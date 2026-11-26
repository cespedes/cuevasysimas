package places

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cespedes/cuevasysimas/internal/access"
)

// ErrInvalidReference is a reference whose key is blank or longer than MaxReferenceKeyLength.
var ErrInvalidReference = errors.New("the key of a reference is required, of at most 200 characters")

// MaxReferenceKeyLength is the longest key a reference may have, in characters.
const MaxReferenceKeyLength = 200

// PlaceRef is a place as a client names it: enough to show it and link to it. A client keeps only the id of
// the places it uses and asks the catalogue for the rest.
type PlaceRef struct {
	ID int64 `json:"id"`
	// Name is its name, and TypeName what it is (the subtype of a cavity, else its type).
	Name     string `json:"name"`
	TypeName string `json:"type_name"`
	// Where is the zone and the ones it is in, from the most specific to the country, without the autonomous
	// community when there is a province: "Valdemanco, Madrid, España".
	Where string `json:"where"`
}

// PlaceRefs names the places with those ids, by name, as the viewer may know them: one that is closed to them
// is left out as if it did not exist, and so is an id that is no place.
func (s *Service) PlaceRefs(ctx context.Context, viewer access.Viewer, ids []int64) ([]PlaceRef, error) {
	refs := []PlaceRef{}
	if len(ids) == 0 {
		return refs, nil
	}
	rows, err := s.repo.PlaceRefs(ctx, ids)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for _, row := range rows {
		if row.visibility.CanSee(viewer, now, access.PartPlace) {
			refs = append(refs, row.PlaceRef)
		}
	}
	return refs, nil
}

// placeRefRow is a reference with the visibility of its place, for the Service to decide who may know it.
type placeRefRow struct {
	PlaceRef
	visibility Visibility
}

// AddReference says that a client uses a place. The key is the client's own (an activity, for the intranet)
// and what makes this use a different one from the client's others; saying it again changes nothing. A place
// that any client uses is not deleted: what was done there stays.
func (s *Service) AddReference(ctx context.Context, placeID int64, issuer, key string) error {
	key = strings.TrimSpace(key)
	if key == "" || utf8.RuneCountInString(key) > MaxReferenceKeyLength {
		return ErrInvalidReference
	}
	return s.repo.AddReference(ctx, placeID, issuer, key)
}

// RemoveReference says that a client no longer uses a place in that way. Saying it of one that is not there
// is no error.
func (s *Service) RemoveReference(ctx context.Context, placeID int64, issuer, key string) error {
	return s.repo.RemoveReference(ctx, placeID, issuer, strings.TrimSpace(key))
}

// HasReferences says whether any client uses the place.
func (s *Service) HasReferences(ctx context.Context, placeID int64) (bool, error) {
	return s.repo.PlaceHasReferences(ctx, placeID)
}
