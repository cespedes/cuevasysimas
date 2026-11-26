-- +goose Up
-- An issuer proves who it vouches for with a signature (internal/assertion), so it has a public key: 32
-- bytes of an Ed25519 key. It is empty only while a client is being registered (cmd/issuer sets it); an
-- issuer with none cannot vouch for anybody.
ALTER TABLE catalogue.issuers
    ADD COLUMN public_key bytea CHECK (public_key IS NULL OR octet_length(public_key) = 32);

-- A client says that it uses a place (an activity went there, for the intranet), and the catalogue does not
-- delete a place that any client uses. The key is the client's own, opaque to the catalogue; a client that
-- stops using a place says so. The foreign key backs the check up for a reference made at the same time as
-- the delete.
CREATE TABLE catalogue.place_references (
    place_id   bigint      NOT NULL REFERENCES catalogue.places (id) ON DELETE RESTRICT,
    issuer     text        NOT NULL REFERENCES catalogue.issuers (issuer) ON DELETE RESTRICT,
    ref_key    text        NOT NULL CHECK (btrim(ref_key) <> '' AND char_length(ref_key) <= 200),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (place_id, issuer, ref_key)
);
CREATE INDEX idx_place_references_issuer ON catalogue.place_references (issuer);

-- +goose Down
DROP TABLE catalogue.place_references;
ALTER TABLE catalogue.issuers DROP COLUMN public_key;
