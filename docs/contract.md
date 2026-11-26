# The contract with the clients

A client of the catalogue is a system that has people of its own (the intranet of the Club de
Espeleología Katiuskas is the first) and wants them to read and change the catalogue. The catalogue
knows nothing of the client's people but what the client says of them in each request.

## Registering a client

A client is an **issuer**: a name (`katiuskas-intranet`), a service account that is shown as its
own, and an Ed25519 public key. It makes its own key pair (`issuer keygen` does, for whoever has no
better tool), keeps the private key and gives the catalogue the public one:

```sh
issuer register -issuer katiuskas-intranet -name "Club Katiuskas intranet" -public-key <base64>
```

Registering again with another key changes it; the catalogue takes the change up within a minute. An
issuer with no key cannot vouch for anybody.

## The assertion

Every request that needs to know who is acting carries `Authorization: Bearer <assertion>`. The
client makes the assertion **for each request** (or for a few seconds' worth of them) from what it
knows of the person who is asking; the catalogue checks it with the public key of the issuer it
names, and calls nobody.

```
v1.<payload>.<signature>
```

The payload is JSON, the signature is that of the bytes `v1.<payload>` made with the private key of
the issuer, and both are base64url with no padding.

| Claim | Meaning |
|---|---|
| `iss` | the issuer: the name the client was registered with |
| `sub` | who the person is for the client: opaque, and the same for the same person every time. It is never the person's name or e-mail, and the catalogue never reads anything from it |
| `name` | how the client calls the person: what is shown as the author of a change. The catalogue keeps the last one it was given |
| `groups` | the groups the person belongs to (`katiuskas` for the members of the club) |
| `roles` | what they are in them (`admin`, `board`…): the catalogue asks for `admin` and `board` on some routes |
| `iat`, `exp` | when it was made and when it stops holding, in seconds of Unix time. At most ten minutes apart; the clocks may differ by thirty seconds |

An assertion that is malformed, signed with another key, expired, or about nobody (no `iss`, `sub`
or `name`) is `401 unauthorized`. One about a person in no group is `403 forbidden` on the routes
for clients: the catalogue's API has nothing to say to them.

A client makes its assertions with a few lines of the standard library (`crypto/ed25519`); the same
lines are in `internal/assertion` (`Sign`), and are copied, not shared.

## What the catalogue does with a person

The first time a person **changes** something (never when they only read) the catalogue makes an
account for them, linked to the identity `(iss, sub)`, and shows its name wherever they are the
author. If the client later calls them something else, the name follows from their next change. The
catalogue never deletes an account that made something.

## What a client keeps, and asks

- **The ids of the places it uses**, and nothing else of them. It asks the names, what each is and
  where it is with `GET /places/refs?ids=1,2,3`: a place that the person may not know exists is not
  in the answer.
- **References.** When the client uses a place (the intranet: an activity went there) it says `PUT
  /places/{id}/references/{key}`, with a key of its own that tells this use from its others
  (`activity:123`), and `DELETE` of the same when it stops. The catalogue does not delete a place
  that any client uses (`409 place_in_use`). A reference is idempotent and belongs to its issuer.
- **What the catalogue cannot know**, the client adds: how many activities went to each place, for
  one.

## What the catalogue never does

It never calls a client, never keeps a session, never asks a client whether a person exists, and has
no table of the client's people. A client can stop being one by deleting its key: from then on its
assertions are refused and what it made stays.

## Routes

The routes are in [`api.md`](api.md) and `openapi/openapi.yaml`. A client uses the API under
`/api/v1` (`/places`, `/systems`, `/areas`, `/osm`, `/places/refs`, the references); everybody uses
the public routes (`/api/v1/public`) and the pages of the site.
