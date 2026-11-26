package places_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/cespedes/cuevasysimas/internal/places"
)

func referencePath(p places.Place, key string) string {
	return placePath(p, "/references/"+key)
}

// A client says which places it uses, and the catalogue does not delete one that any client uses: what was
// done there stays. Saying it twice is saying it once, and a client that stops using a place says so.
func TestAPlaceThatAClientUsesIsNotDeleted(t *testing.T) {
	e := newEnv(t)
	p := e.newPlace(e.cavidad, "Cueva con historia", nil)
	e.notifier.wait(t)

	status(t, e.json("PUT", referencePath(p, "activity-1"), e.member.token, nil), http.StatusNoContent)
	status(t, e.json("PUT", referencePath(p, "activity-1"), e.member.token, nil), http.StatusNoContent) // idempotent
	status(t, e.json("PUT", referencePath(p, "activity-2"), e.member.token, nil), http.StatusNoContent)
	errorCode(t, e.json("DELETE", placePath(p, ""), e.admin.token, nil), http.StatusConflict, "place_in_use")
	if rec := e.json("GET", placePath(p, ""), e.member.token, nil); rec.Code != http.StatusOK {
		t.Errorf("the place must still be there, got %d", rec.Code)
	}

	// One use less is not yet free; both gone, it can be deleted. Removing one that is not there is no error.
	status(t, e.json("DELETE", referencePath(p, "activity-1"), e.member.token, nil), http.StatusNoContent)
	errorCode(t, e.json("DELETE", placePath(p, ""), e.admin.token, nil), http.StatusConflict, "place_in_use")
	status(t, e.json("DELETE", referencePath(p, "activity-2"), e.member.token, nil), http.StatusNoContent)
	status(t, e.json("DELETE", referencePath(p, "activity-2"), e.member.token, nil), http.StatusNoContent)
	status(t, e.json("DELETE", placePath(p, ""), e.admin.token, nil), http.StatusNoContent)
}

// A reference is a key of the client's own: not blank, not long; the place has to be there.
func TestReferencesAreValidated(t *testing.T) {
	e := newEnv(t)
	p := e.newPlace(e.cavidad, "Cueva", nil)
	e.notifier.wait(t)
	errorCode(t, e.json("PUT", referencePath(p, strings.Repeat("a", places.MaxReferenceKeyLength+1)), e.member.token, nil), http.StatusBadRequest, "invalid_reference")
	errorCode(t, e.json("PUT", referencePath(p, "%20"), e.member.token, nil), http.StatusBadRequest, "invalid_reference")
	errorCode(t, e.json("PUT", "/api/v1/places/987654/references/x", e.member.token, nil), http.StatusNotFound, "not_found")
	errorCode(t, e.json("PUT", "/api/v1/places/abc/references/x", e.member.token, nil), http.StatusBadRequest, "invalid_id")
	if rec := e.json("PUT", referencePath(p, "x"), "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("an assertion is needed, got %d", rec.Code)
	}
	if rec := e.json("PUT", referencePath(p, "x"), e.outsider, nil); rec.Code != http.StatusForbidden {
		t.Errorf("somebody in no group: %d, want 403", rec.Code)
	}
}

// The references of a client are the client's: the same key of another issuer is another reference, and one
// client cannot take away the use that another makes of a place.
func TestReferencesBelongToTheirIssuer(t *testing.T) {
	e := newEnv(t)
	p := e.newPlace(e.cavidad, "Cueva", nil)
	e.notifier.wait(t)
	status(t, e.json("PUT", referencePath(p, "k"), e.member.token, nil), http.StatusNoContent)
	// Another client (another issuer), in the same group so that it may know the place.
	var other int64
	if err := e.tx.QueryRow(t.Context(), `INSERT INTO catalogue.accounts (kind, display_name) VALUES ('service', 'Otro') RETURNING id`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if _, err := e.tx.Exec(t.Context(), `INSERT INTO catalogue.issuers (issuer, account_id) VALUES ('other-client', $1)`, other); err != nil {
		t.Fatal(err)
	}
	if _, err := e.tx.Exec(t.Context(), `INSERT INTO catalogue.place_references (place_id, issuer, ref_key) VALUES ($1, 'other-client', 'k')`, p.ID); err != nil {
		t.Fatalf("the same key of another issuer is another reference: %v", err)
	}
	status(t, e.json("DELETE", referencePath(p, "k"), e.member.token, nil), http.StatusNoContent)
	errorCode(t, e.json("DELETE", placePath(p, ""), e.admin.token, nil), http.StatusConflict, "place_in_use")
}

// A client keeps only the ids of the places it uses and asks the names: by name, with what each is and where, as
// the person may know them (a place closed to them, or one that is not there, is not in the answer).
func TestPlaceRefs(t *testing.T) {
	e := newEnv(t)
	sima := e.newPlace(e.cavidad, "Sima Zeta", map[string]any{"subtype_id": e.subSima})
	e.notifier.wait(t)
	cueva := e.newPlace(e.cavidad, "Cueva Alfa", nil)
	e.notifier.wait(t)
	ids := fmt.Sprintf("%d,%d,987654", sima.ID, cueva.ID)

	got := decode[[]places.PlaceRef](t, e.json("GET", "/api/v1/places/refs?ids="+ids, e.member.token, nil))
	if len(got) != 2 || got[0].Name != "Cueva Alfa" || got[0].TypeName != "Cueva" || got[0].Where != "Valdemanco, Madrid, España" || got[1].Name != "Sima Zeta" || got[1].TypeName != "Sima" {
		t.Errorf("by name, with what it is and where: %+v", got)
	}
	if got := decode[[]places.PlaceRef](t, e.json("GET", "/api/v1/places/refs", e.member.token, nil)); len(got) != 0 {
		t.Errorf("no ids, no places: %+v", got)
	}
	errorCode(t, e.json("GET", "/api/v1/places/refs?ids=1,x", e.member.token, nil), http.StatusBadRequest, "invalid_id")
	many := make([]string, places.MaxRefIDs+1)
	for i := range many {
		many[i] = fmt.Sprint(i + 1)
	}
	errorCode(t, e.json("GET", "/api/v1/places/refs?ids="+strings.Join(many, ","), e.member.token, nil), http.StatusBadRequest, "too_many_ids")
	if rec := e.json("GET", "/api/v1/places/refs?ids="+ids, "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("an assertion is needed, got %d", rec.Code)
	}
}
