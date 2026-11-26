# Architecture

The catalogue is one Go server and one PostgreSQL database. It has three faces:

```
 clients (the intranet of the club…)         everybody
   │  Authorization: Bearer <signed assertion>    │
   ▼                                              ▼
 /api/v1/places, /systems, /areas, /osm     /api/v1/public/*   and the pages:  /  /lugares/{name}-{id}
   │  who may see or change what             │   what a visitor may know, nothing else
   ▼                                         ▼
 internal/places  ──  internal/access  ──  internal/publicplaces, internal/publicsite
   │
   ▼
 PostgreSQL, schema `catalogue`
```

## What is where

- **`internal/places`**: the catalogue itself. A place has a type (and a subtype for a cavity), a
  main coordinate, a zone, other points, files, an optional system of connected cavities, and a
  visibility. `redact.go` decides what a viewer may know of a place, and nothing else does.
- **`internal/areas`**: the tree of zones (states and their administrative divisions, seeded).
- **`internal/osmmirror`** and `cmd/osmsync`: a local copy of the OpenStreetMap elements that
  matter, kept apart from the places and joined by element type and id (`osm-integration.md`).
- **`internal/access`**: viewers, audiences, parts, actions and rules. A `Viewer` is an assertion by
  an issuer; `Rules.Allows` is the only question about who may do what.
- **`internal/accounts`**: who acts. An account is a person or a service; a service is registered as
  an issuer with its public key, and a person has an identity (issuer and subject) that makes their
  account the first time they change something.
- **`internal/assertion`** and **`internal/auth`**: the signed statement by which a client says who
  is acting, and the middleware that turns it into a viewer.
- **`internal/publicplaces`** and **`internal/publicsite`**: the public face. Every field they say
  is copied by name from a place that has already been redacted, so what is not copied is never
  said.

## Principles

- **Closed by default.** Every place is for the club's group alone until somebody opens it, at a
  level of its own. A place that is closed does not exist for whoever may not see it: no data, no
  count, a 404.
- **The catalogue never asks the clients.** A person is proved by a signature that can be checked
  alone; it keeps no session and calls nobody. A client keeps the ids of what it uses and asks the
  names (`GET /places/refs`).
- **What is in use is not deleted.** A client says which places it uses (references) and the
  catalogue refuses to delete one that any client uses.
- **No foreign key is ever anything but `ON DELETE RESTRICT`** (but the zone of an element of the
  mirror, which is derived), and every table is in the schema `catalogue`.
- **Tests use a real database**, in a transaction that is rolled back.
