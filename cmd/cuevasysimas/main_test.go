package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/cespedes/cuevasysimas/internal/access"
	"github.com/cespedes/cuevasysimas/internal/accounts"
	"github.com/cespedes/cuevasysimas/internal/assertion"
	"github.com/cespedes/cuevasysimas/internal/config"
	"github.com/cespedes/cuevasysimas/internal/testutil"
)

type healthy struct{}

func (healthy) Ping(context.Context) error { return nil }

// client is a client of the catalogue as the tests play it: it holds the private key that it signs with, and
// its public one is registered in the database.
type client struct {
	t      *testing.T
	tx     pgx.Tx
	priv   ed25519.PrivateKey
	router http.Handler
}

func newClient(t *testing.T, siteURL string) *client {
	t.Helper()
	tx := testutil.NewTx(t)
	ctx := context.Background()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := accounts.NewService(accounts.NewRepository(tx)).RegisterIssuer(ctx, access.IntranetIssuer, "Club Katiuskas intranet", pub); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		PlaceFilesRoot: t.TempDir(), NominatimURL: "off", PublicSiteURL: siteURL,
		PublicSiteLegalName: "Proyecto", PublicSiteLegalAddress: "Calle Falsa 1", PublicSiteLegalEmail: "hola@cuevasysimas.es",
	}
	router, err := buildRouter(cfg, tx, healthy{}, nil)
	if err != nil {
		t.Fatalf("buildRouter: %v", err)
	}
	return &client{t: t, tx: tx, priv: priv, router: router}
}

// seeded are the ids of the seeded type Cavidad, its subtype Cueva and the municipality of Valdemanco.
func (c *client) seeded() (typeID, subtypeID, areaID int64) {
	c.t.Helper()
	ctx := context.Background()
	for _, q := range []struct {
		sql string
		to  *int64
	}{
		{`SELECT id FROM catalogue.place_types WHERE name = 'Cavidad'`, &typeID},
		{`SELECT id FROM catalogue.place_subtypes WHERE name = 'Cueva'`, &subtypeID},
		{`SELECT id FROM catalogue.areas WHERE name = 'Valdemanco' LIMIT 1`, &areaID},
	} {
		if err := c.tx.QueryRow(ctx, q.sql).Scan(q.to); err != nil {
			c.t.Fatal(err)
		}
	}
	return
}

// assertion signs what the client says of a person, as the intranet will for each request of a member.
func (c *client) assertion(subject, name string, groups, roles []string, lifetime time.Duration) string {
	c.t.Helper()
	now := time.Now()
	token, err := assertion.Sign(c.priv, assertion.Claims{
		Issuer: access.IntranetIssuer, Subject: subject, Name: name, Groups: groups, Roles: roles,
		IssuedAt: now.Unix(), ExpiresAt: now.Add(lifetime).Unix(),
	})
	if err != nil {
		c.t.Fatal(err)
	}
	return token
}

func (c *client) member(name string) string {
	return c.assertion("subject-of-"+name, name, []string{access.ClubGroup}, []string{"member"}, 5*time.Minute)
}

func (c *client) do(method, host, path, token string, body any) *httptest.ResponseRecorder {
	c.t.Helper()
	var reader *strings.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = strings.NewReader(string(b))
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, "http://"+host+path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	c.router.ServeHTTP(rec, req)
	return rec
}

// The contract, end to end through the real router and the real signature check: what a client may do with an
// assertion, and what it may not.
func TestAClientActsWithSignedAssertions(t *testing.T) {
	c := newClient(t, "https://cuevasysimas.es")
	const api = "api.cuevasysimas.test"

	typeID, subtypeID, areaID := c.seeded()
	place := map[string]any{"type_id": typeID, "subtype_id": subtypeID, "area_id": areaID, "name": "Cueva del contrato", "lat": 40.85, "lon": -3.65}

	// No assertion, one signed with a key that is not the issuer's, one that has expired, one about a person in
	// no group: refused, each in its own way.
	if rec := c.do("POST", api, "/api/v1/places", "", place); rec.Code != http.StatusUnauthorized {
		t.Errorf("no assertion: %d, want 401", rec.Code)
	}
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	forged, _ := assertion.Sign(other, assertion.Claims{Issuer: access.IntranetIssuer, Subject: "x", Name: "x", Groups: []string{access.ClubGroup},
		IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Minute).Unix()})
	if rec := c.do("POST", api, "/api/v1/places", forged, place); rec.Code != http.StatusUnauthorized {
		t.Errorf("a forged assertion: %d, want 401", rec.Code)
	}
	stale := c.assertion("subject-of-ana", "ana", []string{access.ClubGroup}, nil, -time.Hour)
	if rec := c.do("POST", api, "/api/v1/places", stale, place); rec.Code != http.StatusUnauthorized {
		t.Errorf("an expired assertion: %d, want 401", rec.Code)
	}
	nobody := c.assertion("subject-of-eve", "eve", nil, []string{"admin"}, 5*time.Minute)
	if rec := c.do("POST", api, "/api/v1/places", nobody, place); rec.Code != http.StatusForbidden {
		t.Errorf("a person in no group: %d, want 403", rec.Code)
	}

	// A member of the club creates a place; the catalogue makes an account for them and shows their name.
	rec := c.do("POST", api, "/api/v1/places", c.member("ana"), place)
	if rec.Code != http.StatusCreated {
		t.Fatalf("a member creates a place: %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID        int64  `json:"id"`
		CreatedBy string `json:"created_by"`
		Vis       struct {
			Place string `json:"place"`
		} `json:"visibility"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.CreatedBy != "ana" || created.Vis.Place != "club" {
		t.Errorf("created by the name the issuer gave, and closed: %+v", created)
	}

	// The roles are the issuer's word: a member may not manage the types of place, an admin may.
	if rec := c.do("POST", api, "/api/v1/places/types", c.member("ana"), map[string]any{"name": "Mirador"}); rec.Code != http.StatusForbidden {
		t.Errorf("a member managing types: %d, want 403", rec.Code)
	}
	admin := c.assertion("subject-of-juan", "juan", []string{access.ClubGroup}, []string{"admin"}, 5*time.Minute)
	if rec := c.do("POST", api, "/api/v1/places/types", admin, map[string]any{"name": "Mirador"}); rec.Code != http.StatusCreated {
		t.Errorf("an admin managing types: %d %s", rec.Code, rec.Body.String())
	}

	// The client says that it uses the place: it is not deleted until it says it does not.
	ref := fmt.Sprintf("/api/v1/places/%d/references/activity-1", created.ID)
	if rec := c.do("PUT", api, ref, c.member("ana"), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("a reference: %d %s", rec.Code, rec.Body.String())
	}
	del := fmt.Sprintf("/api/v1/places/%d", created.ID)
	if rec := c.do("DELETE", api, del, admin, nil); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "place_in_use") {
		t.Errorf("a place that is used: %d %s", rec.Code, rec.Body.String())
	}
	if rec := c.do("GET", api, "/api/v1/places/refs?ids="+fmt.Sprint(created.ID), c.member("ana"), nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Cueva del contrato") {
		t.Errorf("the names of the places a client keeps: %d %s", rec.Code, rec.Body.String())
	}
	_ = c.do("DELETE", api, ref, c.member("ana"), nil)
	if rec := c.do("DELETE", api, del, admin, nil); rec.Code != http.StatusNoContent {
		t.Errorf("a place that is no longer used: %d", rec.Code)
	}
}

// The public face, in the same process: anybody reads what is public and the member sees the club's; the
// pages answer only the host of the site.
func TestThePublicFaceAnswersAnybody(t *testing.T) {
	c := newClient(t, "https://cuevasysimas.es")
	const host = "cuevasysimas.es"

	typeID, subtypeID, areaID := c.seeded()
	rec := c.do("POST", host, "/api/v1/places", c.member("ana"), map[string]any{"type_id": typeID, "subtype_id": subtypeID, "area_id": areaID, "name": "Cueva reservada", "lat": 40.85, "lon": -3.65})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}

	if rec := c.do("GET", host, "/api/v1/public/places", "", nil); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "Cueva reservada") {
		t.Errorf("the public list does not have a closed place: %d %s", rec.Code, rec.Body.String())
	}
	if rec := c.do("GET", host, "/api/v1/public/places", c.member("ana"), nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Cueva reservada") {
		t.Errorf("a member sees it: %d %s", rec.Code, rec.Body.String())
	}
	if rec := c.do("GET", host, "/", "", nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Cuevas y simas") || strings.Contains(rec.Body.String(), "Cueva reservada") {
		t.Errorf("the home page: %d", rec.Code)
	}
	if rec := c.do("GET", host, "/robots.txt", "", nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Sitemap: https://cuevasysimas.es/sitemap.xml") {
		t.Errorf("robots.txt: %d %s", rec.Code, rec.Body.String())
	}
	if rec := c.do("GET", "other.example", "/", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("the same page on another host: %d, want 404", rec.Code)
	}
	if rec := c.do("GET", host, "/api/v1/health", "", nil); rec.Code != http.StatusOK {
		t.Errorf("health: %d", rec.Code)
	}
}

// An address for the site that is not an address stops the start, and none leaves the site off.
func TestThePublicSiteAddress(t *testing.T) {
	tx := testutil.NewTx(t)
	bad := config.Config{PlaceFilesRoot: t.TempDir(), NominatimURL: "off", PublicSiteURL: "cuevasysimas.es"}
	if _, err := buildRouter(bad, tx, healthy{}, nil); err == nil {
		t.Error("an address that is not like https://cuevasysimas.es must stop the start")
	}
	off, err := buildRouter(config.Config{PlaceFilesRoot: t.TempDir(), NominatimURL: "off"}, tx, healthy{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	off.ServeHTTP(rec, httptest.NewRequest("GET", "http://cuevasysimas.es/", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("a server with no site address has no site: %d", rec.Code)
	}
}
