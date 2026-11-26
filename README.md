# Cuevas y simas

An open catalogue of caves, shafts, canyons and other underground places: where they are, what they
are and in what state. It is an independent project, with its own domain
(<https://cuevasysimas.es>), API, server and database. Written in Go and PostgreSQL, with HTML and
CSS and no JavaScript on its pages, and no dependency that is not indispensable.

The catalogue was born inside the intranet of the Club de Espeleología Katiuskas, which is its first
client: the members of the club edit the places there, and this project keeps them and shows to
everybody what each place's visibility allows. See [`docs/architecture.md`](docs/architecture.md)
for how it fits together and [`docs/contract.md`](docs/contract.md) for how a client talks to it.

- **A public site**, server-rendered, that search engines can read: the list of places, a page for
  each, `sitemap.xml` and `robots.txt`, with no script, no cookie and no map of anybody's tiles.
- **A public API** (`/api/v1/public`), read-only, that says only what a visitor may know.
- **An API for its clients** (`/api/v1/places`, `/systems`, `/areas`, `/osm`…), where a client
  proves who is acting with an assertion it signs (Ed25519), so that nothing is shared with it but a
  public key.
- **Visibility per place**: three levels (the place, its location, its content), each for a group or
  for anybody, and a lock for places that must never be public. A place that is closed does not
  exist for whoever may not see it.

## Quick start

```sh
docker compose up --build            # PostgreSQL and the server, at http://localhost:64721
export DATABASE_URL=postgres://cuevasysimas:cuevasysimas@localhost:64722/cuevasysimas?sslmode=disable
make migrate-up                      # goose, db/migrations/
go run ./cmd/issuer keygen           # a key pair for a client: keep the private key, register the public one
make register-issuer ISSUER=katiuskas-intranet NAME="Club Katiuskas intranet" KEY=<public key>
```

Tests run against a real PostgreSQL, never mocked:

```sh
docker exec <postgres container> psql -U cuevasysimas -d postgres -c "CREATE DATABASE cuevasysimas_test;"
export TEST_DATABASE_URL=postgres://cuevasysimas:cuevasysimas@localhost:64722/cuevasysimas_test?sslmode=disable
go test -race ./...
```

Without `TEST_DATABASE_URL` the tests that need a database skip. After adding a migration, drop and
recreate the test database. Before a commit: `go fmt`, `go build`, `go vet`, `go test -race`,
`staticcheck` and `govulncheck`.

## Layout

| Path | What it has |
|---|---|
| `cmd/cuevasysimas` | the server |
| `cmd/issuer` | registers a client and its public key; makes key pairs |
| `cmd/osmsync` | loads the OpenStreetMap elements that matter into the local mirror (weekly) |
| `internal/places`, `areas`, `osmmirror` | the catalogue: places, zones, the mirror of OpenStreetMap |
| `internal/access`, `accounts` | who may see what, and who acts (accounts, issuers, identities) |
| `internal/assertion`, `auth` | the signed assertions of the clients, and who is asking |
| `internal/publicplaces`, `publicsite` | the public API and the public pages |
| `db/migrations` | the schema (`catalogue`), with goose |
| `docs/` | architecture, contract, API, database, deployment, roadmap |

## Licence

MIT, see [`LICENSE`](LICENSE).
