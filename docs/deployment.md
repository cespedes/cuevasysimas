# Deployment

The catalogue is one binary and one PostgreSQL database, on the machine that suits it (it need not
be the one of any client). It is built from this repository, reads its settings from the
environment, and is put behind a reverse proxy that does the HTTPS. This page uses
`/opt/cuevasysimas` and the service user `cuevasysimas`; the files it mentions are in `deploy/`.

## 1. PostgreSQL

A database and a role of its own, owner of it (so that the migrations can create the schema):

```sh
sudo -u postgres createuser cuevasysimas --pwprompt
sudo -u postgres createdb cuevasysimas --owner cuevasysimas
```

## 2. Building

```sh
make build      # bin/cuevasysimas, bin/issuer, bin/osmsync
```

or `docker build .` for the image of the server. Copy the binaries to `/opt/cuevasysimas/bin`.

## 3. Migrations and the first client

The schema is applied with [goose](https://github.com/pressly/goose), a tool that is installed
apart:

```sh
export DATABASE_URL='postgres://cuevasysimas:CHANGE-ME@127.0.0.1:5432/cuevasysimas?sslmode=disable'
make migrate-up
```

Nobody can change anything until a client is registered. The client makes its key pair, keeps the
private key and gives the public one (see `contract.md`):

```sh
bin/issuer keygen
bin/issuer register -issuer katiuskas-intranet -name "Club Katiuskas intranet" -public-key <base64>
```

The server takes up a changed key within a minute.

## 4. The service

`deploy/systemd/api.env.example` is the file of settings (`/opt/cuevasysimas/api.env`, readable only
by the service user) and `deploy/systemd/cuevasysimas.service` the unit:

| Variable | Meaning |
|---|---|
| `DATABASE_URL` | the connection (required) |
| `HTTP_ADDR` | where it listens, `:64711` by default; behind the proxy, `127.0.0.1:64711` |
| `PLACE_FILES_ROOT` | where the photos and documents of the places are written (back it up with the database) |
| `PUBLIC_SITE_URL` | the address of the public site, `https://cuevasysimas.es`; empty: no site |
| `PUBLIC_SITE_LEGAL_NAME`, `_ADDRESS`, `_EMAIL` | who answers for the site, for its legal pages; until the three are given the pages say it is pending and ask search engines to stay out |
| `NOMINATIM_URL` | the instance that proposes the zone of a coordinate; `off` turns it off |
| `TELEGRAM_BOT_TOKEN`, `TELEGRAM_LOG_CHAT_ID` | the project's own bot and channel, for notices of changes and errors; leave out to send nothing |

```sh
sudo systemctl enable --now cuevasysimas
```

The server shuts down in an orderly way on SIGTERM (it waits up to 25 seconds for what it is doing).

## 5. The reverse proxy

`deploy/Caddyfile.example`. Caddy looks after the HTTPS and may keep what the server marks as public
(`Cache-Control: public`); it must send the address of the visitor as `X-Forwarded-For`
(`reverse_proxy` does), which is what the limit of requests per address reads (120 a minute). The
site answers only the host of `PUBLIC_SITE_URL`. There are no scripts, no cookies and no maps of
anybody's tiles on its pages, so no consent notice is needed; that has to be said again if one of
them is added. The legal notice and the privacy policy are a standard template that assumes exactly
that, and have to be revised by whoever answers for the site, and again when registration opens.

## 6. The OpenStreetMap mirror

`deploy/systemd/cuevasysimas-osm-sync.service` and its timer load, once a week, the elements of
OpenStreetMap that matter into the mirror (`cmd/osmsync`; it needs `osmium-tool`). Its settings are
in `osm-sync.env.example`; see `osm-integration.md`.

## Backups

The database (`pg_dump`, which includes the accounts and the issuers) and `PLACE_FILES_ROOT`,
together. A file is created once and never rewritten, so a copy of the directory is consistent with
a dump taken after it.
