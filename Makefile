.PHONY: migrate-up migrate-down register-issuer run build test

# Migrations are applied with goose (https://github.com/pressly/goose), a tool that is installed apart and is not a
# dependency of the code. DATABASE_URL says which database.
migrate-up:
	goose -dir db/migrations postgres "$$DATABASE_URL" up

migrate-down:
	goose -dir db/migrations postgres "$$DATABASE_URL" down

# Registers a client of the catalogue, or changes its key:
#   make register-issuer ISSUER=katiuskas-intranet NAME="Club Katiuskas intranet" KEY=<base64 public key>
register-issuer:
	go run ./cmd/issuer register -issuer "$(ISSUER)" -name "$(NAME)" -public-key "$(KEY)"

run:
	go run ./cmd/cuevasysimas

build:
	go build -o bin/cuevasysimas ./cmd/cuevasysimas
	go build -o bin/issuer ./cmd/issuer
	go build -o bin/osmsync ./cmd/osmsync

# The tests run against a real PostgreSQL (TEST_DATABASE_URL), never mocked; they apply db/migrations to it
# the first time. After adding a migration drop and recreate that database.
test:
	go test -race ./...
