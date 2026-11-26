# API

The routes of the catalogue of places, for its clients and for the public. The contract with the
clients (who may call, how a person is proved, what a client keeps and asks) is in
[`contract.md`](contract.md); `openapi/openapi.yaml` is the machine-readable form of what is here.

Contents:

- [Conventions](#conventions)
- [Clients: references and names](#clients-references-and-names)
- [Places](#places)
- [Public places](#public-places)
- [Systems](#systems)
- [OpenStreetMap elements](#openstreetmap-elements)
- [Zones](#zones)
- [Objects](#objects)

## Conventions

- **Base**: all routes hang from `/api/v1`. Bodies and responses are JSON, except the bytes of a
  file and the upload of a file, which is `multipart/form-data`.
- **Who asks**: a client proves who is acting with an *assertion* that it signs (`Authorization:
  Bearer <assertion>`, see `contract.md`). "Any person in a group" means a valid assertion about
  somebody who belongs to at least one group; the roles that some routes ask for (`admin`, `board`)
  are those that the client's assertion says. The public routes and the pages need none, and give a
  person who sends one what their groups may see.
- **Dates**: a "date" field is sent as `YYYY-MM-DD`, and a "datetime" field is a real timestamp.
- **Errors**: `{"error": {"code": "…", "message": "…"}}`, with the matching HTTP code. A malformed
  JSON is always `400 invalid_body` and an unexpected failure `500 internal_error`; they are not
  repeated on each route. A missing or invalid assertion is `401 unauthorized`, and one about
  somebody in no group, or who lacks the role a route asks for, is `403 forbidden`.
- **Methods**: `GET` reads and changes nothing; `POST` creates in a collection (`201`) or runs an
  action; `PUT` replaces the whole resource at a URL that is already known (what is not sent is not
  kept) and repeating it leaves the same result; `PATCH` changes only the fields that are sent;
  `DELETE` removes the resource (`204`, no body), and one that is in use is refused with `409`
  instead of being removed with what depends on it.
- **Notation**: in bodies, a `*` marks the required field.

## Clients: references and names

A client keeps only the ids of the places it uses (the intranet of the club: where an activity went)
and asks the catalogue for the rest. These routes are for that, and need an assertion about somebody
in a group who may know the place.

- **GET** `/places/refs` — *Any person in a group*. The names of the places with the ids of `?ids=`
  (numbers separated by commas, at most 200), by name, as the person that the assertion is about may
  know them: a place that is closed to them, or that does not exist, is not in the answer.
  - returns: 200 `[PlaceRef]`
  - errors: 400 `invalid_id`, `too_many_ids`; 401 `unauthorized`
- **PUT** `/places/{id}/references/{key}` — *Any person in a group*. Says that the client the
  assertion comes from uses the place in the way that `key` says (its own opaque text, at most 200
  characters: `activity:123`). Saying it again changes nothing. A place that any client uses is not
  deleted.
  - returns: 204
  - errors: 400 `invalid_id`, `invalid_reference`; 404 `not_found` (also for a place closed to them)
- **DELETE** `/places/{id}/references/{key}` — *Any person in a group*. Says that the client no
  longer uses the place in that way. One that was not there is no error.
  - returns: 204
  - errors: 400 `invalid_id`

### `PlaceRef`

| Field | Type |
|---|---|
| `id` | integer |
| `name` | text |
| `type_name` | text: the subtype of a cavity ("Sima"), else its type |
| `where` | text: the zone and the ones it is in, from the most specific to the country |

## Places

The catalogue of places: caves and shafts, canyons, via ferratas, huts… A place has a type, one main
coordinate and any number of other points. Any person in a group reads, creates and changes places
and their points (the API says who created each place and who changed it last, and when); deleting a
place is for admin and board, and the types of place and the geologies of the caves are managed by
admin alone. Coordinates are WGS84 latitude and longitude, in degrees, and an altitude, when it is
known, in metres above sea level.

- **GET** `/places/types` — *Any person in a group*. The types of place (the cavities, the canyons
  and then the general ones, each in the order they were added), each with its `kind` (`cave` for
  Cavidad, `canyon` for Cañón, `general` for the rest), whether its places have a state
  (`has_state`) and the number of places that have it.
  - returns: 200 `[PlaceType]`
- **POST** `/places/types` — *Admin*. Adds a general type (Cavidad and Cañón are fixed).
  - body: `name*` (unique ignoring case), `has_state` (false if left out: whether its places have a
    state; it cannot be changed afterwards)
  - returns: 201 `PlaceType`
  - errors: 400 `invalid_body`, `place_type_name_required`; 409 `duplicate_place_type`
- **PUT** `/places/types/{id}` — *Admin*. Renames a general type.
  - body: `name*`
  - returns: 200 `PlaceType`
  - errors: as creating, plus 400 `invalid_id`; 404 `not_found`; 409 `fixed_place_type`
- **DELETE** `/places/types/{id}` — *Admin*. Deletes a general type no place has.
  - returns: 204
  - errors: 400 `invalid_id`; 404 `not_found`; 409 `fixed_place_type`, `place_type_in_use`
- **GET** `/places/subtypes` — *Any person in a group*. What a cavity is (Cueva, Sima, Torca, Mina,
  Abrigo and whatever has been added), in the order they were added, each with the number of places
  that have it.
  - returns: 200 `[PlaceSubtype]`
- **POST** `/places/subtypes` — *Admin*. Adds a subtype.
  - body: `name*` (unique ignoring case)
  - returns: 201 `PlaceSubtype`
  - errors: 400 `invalid_body`, `subtype_name_required`; 409 `duplicate_subtype`
- **PUT** `/places/subtypes/{id}` — *Admin*. Renames a subtype (none is fixed).
  - body: `name*`
  - returns: 200 `PlaceSubtype`
  - errors: as creating, plus 400 `invalid_id`; 404 `not_found`
- **DELETE** `/places/subtypes/{id}` — *Admin*. Deletes a subtype no place has.
  - returns: 204
  - errors: 400 `invalid_id`; 404 `not_found`; 409 `subtype_in_use`
- **GET** `/places/geologies` — *Any person in a group*. What a cave can be formed in (Caliza, Yeso,
  Hielo, Lava, Dolomía, Mármol, Sal, Arenisca, Cuarcita, Conglomerado, Granito and whatever has been
  added), in the order they were added, each with the number of places that have it.
  - returns: 200 `[PlaceGeology]`
- **POST** `/places/geologies` — *Admin*. Adds a geology.
  - body: `name*` (unique ignoring case)
  - returns: 201 `PlaceGeology`
  - errors: 400 `invalid_body`, `geology_name_required`; 409 `duplicate_geology`
- **PUT** `/places/geologies/{id}` — *Admin*. Renames a geology (none is fixed).
  - body: `name*`
  - returns: 200 `PlaceGeology`
  - errors: as creating, plus 400 `invalid_id`; 404 `not_found`
- **DELETE** `/places/geologies/{id}` — *Admin*. Deletes a geology no place has.
  - returns: 204
  - errors: 400 `invalid_id`; 404 `not_found`; 409 `geology_in_use`
- **GET** `/places` — *Any person in a group*. Every place, by name (ignoring case), unpaginated,
  with what a table and its filters need: no long texts and no other points (those are in `GET
  /places/{id}`), and where it is (`where`, `area_ids`).
  - returns: 200 `[PlaceListItem]`
- **POST** `/places` — *Any person in a group*. Adds a place.
  - body: `type_id*`, `name*`, `lat*` and `lon*` (the main coordinate: -90 to 90 and -180 to 180)
    and `altitude_m` (its altitude in metres above sea level, -500 to 9000, optional), `alt_names`
    (a list of the other names it is known by: trimmed, without blanks or repeats ignoring case, 20
    at most of 200 characters each), `area_id*` (the most specific zone it is in, of `/areas`: at
    least the country), `access` (how to get there), `description`, and what depends on the type:
    `state` only for a type that has one (`has_state`: Cavidad, Cañón and Vía ferrata do), `open` by
    default (it can be visited freely; `permit_required`: a permit has to be asked for; `tourist`:
    it is run as an attraction and has to be paid for; `closed`); `subtype_id*` (of
    `/places/subtypes`: cueva, sima…), `system_id` (of `/systems`: the system it is in, optional),
    `depth_m` and `horizontal_length_m` (metres) and `geology_id` (of `/places/geologies`: what it
    is formed in) only for a cavity, and `technical_sheet` (what has to be installed to go through
    it) for a cavity or a canyon. Blank text counts as no value. `osm_type` and `osm_id` (both or
    neither: `node`, `way` or `relation` and its id) make the place an element of the OpenStreetMap
    mirror (`GET /osm/elements`), which has to be in it and at most one place may be; only creating
    reads them, an edit leaves them as they were. The element then stops being offered as a
    candidate on the map.
  - returns: 201 `Place`
  - errors: 400 `invalid_body`, `place_name_required`, `place_type_required`, `unknown_place_type`,
    `place_area_required`, `unknown_area`, `invalid_place_state`, `invalid_coordinates`,
    `invalid_altitude`, `unknown_geology`, `place_subtype_required` (a cavity with no subtype),
    `unknown_subtype`, `invalid_alt_names`, `unknown_system` (a `system_id` that is not there),
    `invalid_place_measure` (negative, or above 20000 m of depth or 2000000 m of length),
    `invalid_place_fields` (a subtype, a measure or a geology of a place that is not a cavity, a
    technical sheet of one that is neither a cavity nor a canyon, or a state of a type that has
    none), `unknown_osm_element` (not in the mirror, or only the type or only the id); 409
    `osm_element_in_use` (another place already is that element)
- **GET** `/places/{id}` — *Any person in a group*. One place with its points.
  - returns: 200 `Place`
  - errors: 400 `invalid_id`; 404 `not_found`
- **PUT** `/places/{id}` — *Any person in a group*. Replaces every field of the place (what is left
  out is cleared), its type included, which may be any: the fields have to fit the kind of the new
  one. Its other points are not touched.
  - body: the same as creating
  - returns: 200 `Place`
  - errors: as creating, plus 400 `invalid_id`; 404 `not_found`
- **PATCH** `/places/{id}` — *Any person in a group*. Changes only the fields the body names and
  leaves the rest as it is: a field that is left out is not touched, and one given as `null` is
  cleared (if it may be empty). The place that results has to be valid, with the same checks as a
  whole edit (so taking the subtype off a cavity is refused, and so is a new type whose kind does
  not fit the fields that stay). It is made on the place as it is when it is saved: if somebody
  saves the place between the read and the write, it starts again on the new one, up to three times.
  No `osm_type` or `osm_id` (the element is changed with its own routes).
  - body: any of the fields of creating, at least one
  - returns: 200 `Place`
  - errors: as creating, plus 400 `invalid_id`, `invalid_body` (malformed, no field, an unknown
    field or one of the wrong type); 404 `not_found`; 409 `concurrent_update`
- **DELETE** `/places/{id}` — *Admin or board*. Deletes a place with its points and its files,
  unless a client uses it (see "Clients: references and names").
  - returns: 204
  - errors: 400 `invalid_id`; 404 `not_found`; 409 `place_in_use`
- **PUT** `/places/{id}/settings/visibility` — *Admin or board*. Says how open the place is: the
  three levels and the lock, all at once (the path is not `/places/{id}/visibility`, which would
  clash with `/places/types/{id}` in the router). A place is `club` at every level until somebody
  opens it. It is a change of the place like any other (`updated_by`, `updated_at`) and it is
  announced.
  - body: `place*`, `location*`, `content*` (each `club` or `public`) and `protected*` (a boolean):
    see `PlaceVisibility`
  - returns: 200 `Place`
  - errors: 400 `invalid_id`, `invalid_body`, `invalid_place_visibility` (a level that is neither),
    `visibility_too_open` (the location or the content more open than the place), `place_protected`
    (something public on a protected place); 403 `forbidden`; 404 `not_found`
- **PUT** `/places/{id}/links/osm` — *Any person in a group*. Says which element of the
  OpenStreetMap mirror the place is, instead of the one it was, if any: nothing else of the place
  changes (but `updated_by` and `updated_at`). The element stops being a candidate on the map. (The
  path is not `/places/{id}/osm`, which would clash with `/places/types/{id}` in the router.)
  - body: `osm_type*` (`node`, `way` or `relation`), `osm_id*`
  - returns: 200 `Place`
  - errors: 400 `invalid_id`, `invalid_body`, `unknown_osm_element` (not in the mirror, or a type
    that is not one, or an id that is not positive); 404 `not_found`; 409 `osm_element_in_use`
    (another place already is that element)
- **DELETE** `/places/{id}/links/osm` — *Any person in a group*. Says the place is not an element of
  OpenStreetMap any more; the element stays in the mirror, and is a candidate again. Doing it on a
  place that has none is no error.
  - returns: 200 `Place`
  - errors: 400 `invalid_id`; 404 `not_found`
- **POST** `/places/{id}/links/osm/review` — *Any person in a group*. Says that somebody has looked
  at what changed in the element of the place: what the element is now becomes what the place has
  seen of it, so `osm_changes` is empty until it changes again. Nothing else changes, not even
  `updated_at`.
  - returns: 200 `Place`
  - errors: 400 `invalid_id`; 404 `not_found`; 409 `place_not_linked`
- **POST** `/places/{id}/points` — *Any person in a group*. Adds one of the other points of a place.
  Adding, changing or removing one counts as a change of the place (`updated_by`, `updated_at`).
  - body: `kind*` (`parking`, `approach`, `waypoint`, `access` or `other`), `lat*`, `lon*`,
    `altitude_m` (-500 to 9000, optional), `name`, `note`
  - returns: 201 `PlacePoint`
  - errors: 400 `invalid_id`, `invalid_body`, `invalid_point_kind`, `invalid_coordinates`,
    `invalid_altitude`; 404 `not_found`
- **PUT** `/places/{id}/points/{pointId}` — *Any person in a group*. Replaces every field of a
  point.
  - body: the same as creating
  - returns: 200 `PlacePoint`
  - errors: as creating; 404 `not_found` (also for a point of another place)
- **DELETE** `/places/{id}/points/{pointId}` — *Any person in a group*. Removes a point.
  - returns: 204
  - errors: 400 `invalid_id`; 404 `not_found`
- **GET** `/places/{id}/files` — *Any person in a group*. The files of a place (photos,
  topographies, permits, documents), oldest first.
  - returns: 200 `[PlaceFile]`
  - errors: 400 `invalid_id`; 404 `not_found`
- **POST** `/places/{id}/files` — *Any person in a group*. Attaches a file. Like a point, it counts
  as a change of the place (`updated_by`, `updated_at`).
  - body: multipart form, `file*` (10 MiB at most) and `title` (a caption, 200 characters at most)
  - returns: 201 `PlaceFile`
  - errors: 400 `invalid_id`, `invalid_body`, `file_required`, `file_title_too_long`; 404
    `not_found`; 413 `file_too_large`
- **GET** `/places/{id}/files/{fileId}/file` — *Any person in a group*. The bytes. A PDF or an image
  (decided from its bytes) is shown by the browser, anything else is downloaded and sandboxed;
  `?download=1` forces a download.
  - errors: 400 `invalid_id`; 404 `not_found`
- **DELETE** `/places/{id}/files/{fileId}` — *Whoever attached it, admin or board*.
  - returns: 204
  - errors: 400 `invalid_id`; 403 `place_file_not_yours`; 404 `not_found`

## Public places

The read-only face of the catalogue, for people who are not signed in (the site «Cuevas y simas», at
its own domain, is made of it). It is a group of routes and a package of its own
(`internal/publicplaces`), so what it says is only what is written here. No token is anonymous; a
valid bearer token is the person, who sees what their visibility allows (a member of a group sees
more); an invalid one is 401, never the public answer.

- **GET** `/public/places` — *Anybody*. The places that the viewer may know exist, by name. A place
  whose level `place` is closed to them is not in it (see `PlaceVisibility`).
  - returns: 200 `[PublicPlaceListItem]`
  - errors: 401 `unauthorized`; 429 `rate_limited`
- **GET** `/public/places/{id}` — *Anybody*. One place as the viewer may see it. One that is closed
  to them and one that does not exist give the same 404.
  - returns: 200 `PublicPlace`
  - errors: 400 `invalid_id`; 401 `unauthorized`; 404 `not_found`; 429 `rate_limited`

Each address may ask 120 times a minute (`PublicRateLimit` and `PublicRateWindow`, in the
configuration). Without a token the answer carries `Cache-Control: public, max-age=60,
stale-while-revalidate=300`, so that a cache in front may keep it; with one, `private, no-store`;
and it always has `Vary: Authorization`.

### `PublicPlaceListItem`

| Field | Type |
|---|---|
| `id` | integer |
| `name` | text |
| `alt_names` | `[text]` |
| `type` | text: what it is (Cavidad, Cañón…) |
| `subtype` | text: what a cavity is (cueva, sima…); left out for the rest |
| `state` | `open`, `permit_required`, `tourist` or `closed`; left out if its type has none |
| `where` | text: the zone and the ones it is in, from the most specific to the country |
| `zone_ids` | `[integer]`: those zones, the country first |
| `measures` | `{depth_m, horizontal_length_m, geology}`; left out if it has none |
| `coordinates` | `{lat, lon, altitude_m}`; **left out** if the viewer may not see the location |

### `PublicPlace`

Everything of a `PublicPlaceListItem`, and:

| Field | Type |
|---|---|
| `zones` | `[{id, kind, name}]`: the zone and the ones it is in, the country first |
| `location` | `{points: [{kind, name, lat, lon, altitude_m, note}], access}`; left out if the viewer may not see the location |
| `content` | `{description, technical_sheet}`; left out if the viewer may not see the content |

A part that depends on the visibility is **absent**, never empty or zero, when the viewer may not
see it. Never in these answers, whatever the place has: who wrote it and when, the OpenStreetMap
element, the visibility itself, the files, the system it is in and anything of a client's own
identifiers.

## Systems

A system is a group of cavities that are connected underground: a name, a description and files of
its own, and the cavities that form it (each is a place of its own, in at most one system, and only
a cavity can be in one). Any person in a group reads, creates and changes systems and sets their
cavities and files; deleting one is for admin and board.

- **GET** `/systems` — *Any person in a group*. Every system by name (ignoring case), each with its
  cavities by name.
  - returns: 200 `[SystemListItem]`
- **POST** `/systems` — *Any person in a group*. Creates a system.
  - body: `name*` (at most 200 characters, unique ignoring case), `description`
  - returns: 201 `System`
  - errors: 400 `invalid_body`, `system_name_required`, `system_name_too_long`; 409
    `duplicate_system`
- **GET** `/systems/{id}` — *Any person in a group*. One system with its cavities, each with
  everything about it (what `GET /places` has of a place and its `access`, `description`,
  `technical_sheet`, its own `files` and its other `points`).
  - returns: 200 `System`
  - errors: 400 `invalid_id`; 404 `not_found`
- **PUT** `/systems/{id}` — *Any person in a group*. Replaces the name and the description.
  - body: the same as creating
  - returns: 200 `System`
  - errors: as creating, plus 400 `invalid_id`; 404 `not_found`
- **DELETE** `/systems/{id}` — *Admin or board*. Deletes a system with its files; its cavities stay,
  in no system.
  - returns: 204
  - errors: 400 `invalid_id`; 404 `not_found`
- **PUT** `/systems/{id}/places` — *Any person in a group*. Makes the cavities of the system exactly
  the given ones: the others are left in no system, and one that was in another system moves to
  this. A repeated id counts once, and a refused change changes nothing.
  - body: `place_ids*`
  - returns: 200 `[SystemPlaceRef]`, the cavities as they are now
  - errors: 400 `invalid_id`, `invalid_body`, `unknown_place`, `not_a_cavity`; 404 `not_found`
- **GET** `/systems/{id}/files`, **POST** `/systems/{id}/files`, **GET**
  `/systems/{id}/files/{fileId}/file` and **DELETE** `/systems/{id}/files/{fileId}` — *As the files
  of a place*: the photos and documents of the system, with the same body, limits, errors and
  permissions (whoever attached a file, admin and board remove it). They are `PlaceFile`s with a
  `system_id`.

A cavity says which system it is in with `system_id` when it is created or edited (`POST` and `PUT
/places`), and `GET /places` and `GET /places/{id}` give `system_id` and `system`.

## OpenStreetMap elements

Read-only access to the local mirror of OpenStreetMap (`docs/osm-integration.md`): the caves,
canyons, mines, huts and via ferratas that `cmd/osmsync` loads from Geofabrik's extracts. Any person
in a group reads; nothing here changes the mirror, and nothing in these requests goes to
OpenStreetMap.
  - errors: 400 `invalid_id`

- **GET** `/osm/elements` — *Any person in a group*. The elements inside a box, for a map: those not
  marked as gone, the catalogue's own subject first (caves, canyons and mines, then huts and via
  ferratas), at most 1,500; `truncated` says there were more, so the box is too wide. The answer is
  kept by the browser for five minutes.
  - query: `bbox*` (`south,west,north,east`, in degrees; a `west` greater than `east` is a box
    across the antimeridian), `category` (a comma-separated list of `cave`, `canyon`, `mine`, `hut`,
    `via_ferrata`; all if left out)
  - returns: 200 `{elements: [OsmPoint], truncated}`, an empty list when there is nothing
  - errors: 400 `invalid_bbox` (missing, not four numbers, out of range, or south above north); 400
    `invalid_osm_category`
- **GET** `/osm/elements/{type}/{id}` — *Any person in a group*. One element with all its tags and
  its outline, which a map asks for when the point is chosen.
  - returns: 200 `OsmElement`
  - errors: 400 `invalid_osm_element` (a type that is not `node`, `way` or `relation`), 400
    `invalid_id`; 404 `not_found`
- **GET** `/osm/nearby` — *Any person in a group*. The elements near a point that are not yet a
  place, for linking a place of the catalogue to one (`PUT /places/{id}/links/osm`): those whose
  name looks like `name` first (`exact`: the same name ignoring case, accents and punctuation;
  `similar`: one holds the other, or they share a word that is not a generic one like "cueva" or
  "sima"), then the nearest, at most 30.
  - query: `lat*`, `lon*`, `radius` (metres, 500 by default, 2000 at most), `name`, `category` (as
    in `/osm/elements`, all if left out)
  - returns: 200 `{elements: [OsmCandidate]}`
  - errors: 400 `invalid_bbox` (the point is not valid), `invalid_radius`, `invalid_osm_category`
- **GET** `/osm/search` — *Any person in a group*. The elements whose name holds the words of a
  text, to find the one a person knows the name of and make a place of it. Case, accents and
  punctuation do not count ("Cova d'Arta" is "cova d arta"), the words may come in any order, and
  the other names of an element (`alt_name`, `name:es`) count too. The best come first: the same
  name, then those that start with the text, then those that have all its words, then those that
  only hold them inside another word, and in each the catalogue's own subject first (caves, canyons,
  mines, huts, via ferratas) and the shorter name; at most 20. The elements that are already a place
  are left out, and so are those gone from OpenStreetMap.
  - query: `q*` (at most 200 characters; one letter, or nothing, finds nothing), `category` (as in
    `/osm/elements`, all if left out)
  - returns: 200 `{elements: [OsmPoint]}`
  - errors: 400 `invalid_osm_query` (too long), `invalid_osm_category`

## Zones

The tree of zones that says where a place is: the 195 states of the United Nations (its 193 members
and its two observers, all seeded) and, below each, whatever its administrative divisions are. For
Spain they are seeded too: the autonomous communities, the provinces and the municipalities of the
INE. A territory that is not a state (Gibraltar, Bermuda, Greenland…) is seeded too, with kind
"Territorio" and its ISO code, below the state that administers it; Antarctica, Western Sahara and
Taiwan, which no state administers or whose administrator is disputed, are zones of that kind with
no zone above them. Nothing in the API knows the levels: each zone has a `kind`, a text that names
its level ("Provincia"), and a place points at the most specific zone it is in. Any person in a
group reads the zones and adds one below another when it is missing; renaming, merging and deleting
them is for admin alone.

- **GET** `/areas` — *Any person in a group*. Three questions, by the query:
  - nothing: the countries, by name;
  - `?parent_id=`: the zones directly below one, by name (404 `not_found` if there is no such zone);
  - `?search=` (with `&limit=`, 20 by default and 50 at most): the zones whose name contains the
    text, ignoring case, accents and punctuation ("coruna" finds "A Coruña"), the exact ones first,
    each with its `ancestors`. An empty text finds nothing.
  - returns: 200 `[Area]`
  - errors: 400 `invalid_id` (a `parent_id` that is not a number); 404 `not_found`
- **GET** `/areas/locate` — *Any person in a group*. Proposes the zone a point is in, asking a
  Nominatim instance (OpenStreetMap's geocoder): the municipality by the INE code of its `ref:ine`
  (only in Spain) when the seed has it, else the country by its ISO code. It is a help, not a fact;
  the answer is kept for a day, and the requests to the instance are spaced a second apart.
  - query: `lat*`, `lon*`
  - returns: 200 `{area}`, an `Area` with its `ancestors`, or `null` when none is known or the
    lookup is off (`NOMINATIM_URL=off`)
  - errors: 400 `invalid_coordinates`; 502 `locate_unavailable` (the instance did not answer)
- **GET** `/areas/geocode` — *Any person in a group*. Looks for places by name (a town, a valley, a
  peak) to find them on a map, asking the same Nominatim instance as `/areas/locate`: at most 6,
  best first. The answers are kept for a day by the text asked, the requests to the instance are
  spaced a second apart, and only the text goes out.
  - query: `q` (at most 200 characters; blank finds nothing)
  - returns: 200 `[{name, lat, lon, bounds}]`, `bounds` being `{south, west, north, east}` or
    `null`; an empty list when nothing is found or the lookup is off (`NOMINATIM_URL=off`)
  - errors: 400 `area_text_too_long`; 502 `search_unavailable` (the instance did not answer)
- **GET** `/areas/{id}` — *Any person in a group*. One zone with its `ancestors`, the country first.
  - returns: 200 `Area`
  - errors: 400 `invalid_id`; 404 `not_found`
- **POST** `/areas` — *Any person in a group; admin to add a country or a zone in Spain*. Adds a
  zone below another or, without `parent_id` (or with 0), a country: a zone in no other one, a
  "País" unless a `kind` says otherwise. A zone in Spain, at any depth, is only added by an admin:
  they come from the INE's lists.
  - body: `parent_id`, `name*` (at most 200 characters; unique below the same zone ignoring case,
    accents and punctuation), `kind` (at most 60 characters). Without a `kind` it is the kind of the
    zones already below that one; with none below it yet, it has to be given.
  - returns: 201 `Area`
  - errors: 400 `invalid_body`, `unknown_area`, `area_name_required`, `area_kind_required`,
    `area_text_too_long`; 403 `area_root_admin_only`, `area_spain_admin_only`; 409 `duplicate_area`
- **PUT** `/areas/{id}` — *Admin*. Renames a zone, if a `kind` is given changes it (a country can be
  renamed) and, if `parent_id` is in the body, moves it with all that is below it below another zone
  (`null` makes it a country); without `parent_id` it stays where it is.
  - body: `name*`, `kind`, `parent_id`
  - returns: 200 `Area`
  - errors: 400 `invalid_id`, `invalid_body`, `unknown_area`, `area_name_required`,
    `area_text_too_long`; 404 `not_found`; 409 `duplicate_area` (the name is taken where it goes),
    `area_move_cycle` (into itself or into a zone below it)
- **DELETE** `/areas/{id}` — *Admin*. Deletes a zone with no zones below it and no places in it.
  - returns: 204
  - errors: 400 `invalid_id`; 404 `not_found`; 409 `area_has_children`, `area_in_use`
- **POST** `/areas/{id}/merge` — *Admin*. Moves the zones below this one and the places in it to
  another zone below the same one (two countries count as below the same one), and deletes this one.
  If the zone that stays has no official code and this one has, it takes it.
  - body: `into_id*`
  - returns: 200 `{id}`: the zone it was merged into
  - errors: 400 `invalid_id`, `invalid_body`, `area_merge_self`; 404 `not_found`; 409
    `area_merge_parents` (they are not below the same zone), `duplicate_area` (a zone below this one
    has the name of one below the other)

## Objects

### `Error`

| Field | Type |
|---|---|
| `error` | object |

Nested fields:

- `error`: `{code, message}`

### `Area`

| Field | Type |
|---|---|
| `id` | integer |
| `parent_id` | integer |
| `kind` | text |
| `name` | text |
| `code` | text |
| `created_by` | text |
| `child_count` | integer |
| `place_count` | integer |
| `ancestors` | `[AreaRef]` |

`parent_id` is absent for a country. `code` is an official code when there is one: ISO 3166-1
alpha-2 for a country, the INE's for a Spanish community, province or municipality. `created_by` is
the name of the account of whoever added the zone (`POST /areas`); it is absent for the zones that
were loaded with the catalogue. `child_count` and `place_count` are the zones and the places
directly below and in the zone, not in all its descendants. `ancestors`, the country first and
without the zone itself, are given by a search and by `GET /areas/{id}`, not by the list of the
zones below one.

### `AreaRef`

| Field | Type |
|---|---|
| `id` | integer |
| `kind` | text |
| `name` | text |

### `PlaceType`

| Field | Type |
|---|---|
| `id` | integer |
| `name` | text |
| `kind` | cave/canyon/general |
| `has_state` | boolean |
| `place_count` | integer |

### `PlaceSubtype`

| Field | Type |
|---|---|
| `id` | integer |
| `name` | text |
| `place_count` | integer |

### `PlaceGeology`

| Field | Type |
|---|---|
| `id` | integer |
| `name` | text |
| `place_count` | integer |

### `PlaceListItem`

| Field | Type |
|---|---|
| `id` | integer |
| `type_id` | integer |
| `type_name` | text |
| `kind` | cave/canyon/general |
| `name` | text |
| `alt_names` | `[text]`, empty if there are none |
| `state` | open/permit_required/tourist/closed, absent if the type has none |
| `subtype_id` | integer, only a cavity |
| `subtype` | text, with `subtype_id` |
| `system_id` | integer, only a cavity in a system |
| `system` | text, with `system_id` |
| `area_id` | integer |
| `area_ids` | `[integer]` |
| `where` | text |
| `area_names` | text |
| `lat` | number |
| `lon` | number |
| `altitude_m` | integer |
| `depth_m` | integer |
| `horizontal_length_m` | integer |
| `geology_id` | integer |
| `geology` | text |
| `osm_changed` | true if the OpenStreetMap element the place is has changed since it was linked or last reviewed; left out otherwise |
| `visibility` | `PlaceVisibility` |
| `withheld` | `[text]`, the parts that were left out for this viewer; left out when none (always, for now) |

### `PlaceVisibility`

How open a place is, in three levels, each `club` (the members of the group `katiuskas` alone) or
`public`:

| Field | Type |
|---|---|
| `place` | `club`/`public`: the place itself, with its name, type, state, zone and measures |
| `location` | `club`/`public`: the main coordinate, the other points and how to get there |
| `content` | `club`/`public`: the description, the technical sheet and the files |
| `protected` | boolean: locks the place closed, nothing of it may be public |

Where `place` is closed, whoever may not see it does not know it exists: it is not in the lists and
reading it is a 404. `location` and `content` cannot be more open than `place`; a protected place
has all three at `club`. Who wrote it is never public. A person of the group `katiuskas` sees every
part, so for them `withheld` is never there; the parts are `place`, `measures`, `coordinates`,
`points`, `access`, `sheet`, `files` and `authors`.

### `Place`

Everything of a `PlaceListItem`, and:

| Field | Type |
|---|---|
| `access` | text |
| `description` | text |
| `technical_sheet` | text |
| `osm_type` | node/way/relation, left out if the place is not an element of OpenStreetMap |
| `osm_id` | integer, with `osm_type` |
| `osm_changes` | `[OsmChange]`: what is different in the element now from when the place was linked or last reviewed; left out if nothing |
| `area_path` | `[AreaRef]` |
| `created_by` | text (the name of the account) |
| `created_at` | datetime |
| `updated_by` | text (the name of the account) |
| `updated_at` | datetime |
| `points` | `[PlacePoint]` |
| `file_count` | number |

`area_id` is the most specific zone it is in; `area_ids` are that zone and the ones it is in, the
country first, so that a list can be filtered by any of them; `where` says them from the most
specific one to the country, without the autonomous community when there is a province, since in
Spain the province says it already ("Valdemanco, Madrid, España"); `area_names` are the names of all
of them, the community too ("Valdemanco, Madrid, Comunidad de Madrid, España"), for a search to look
in; `area_path` is the same as references, the country first and with each `kind`. `kind` says what
the place may have: `depth_m`, `horizontal_length_m` and `geology_id` (with its name in `geology`)
only when it is `cave`, `technical_sheet` when it is `cave` or `canyon`. `altitude_m`, optional, is
that of the main coordinate (the other points have their own). `updated_by` and `updated_at` are the
last change of the place, of any of its points or of its files (the same as the creation until there
is one).

### `SystemListItem`

| Field | Type |
|---|---|
| `id` | integer |
| `name` | text |
| `places` | `[SystemPlaceRef]` |

### `SystemPlaceRef`

| Field | Type |
|---|---|
| `id` | integer |
| `name` | text |
| `subtype` | text |

### `System`

| Field | Type |
|---|---|
| `id` | integer |
| `name` | text |
| `description` | text |
| `created_by`, `updated_by` | text (the name of the account); the last change of the system, its cavities or its files |
| `created_at`, `updated_at` | datetime |
| `file_count` | integer |
| `places` | `[PlaceListItem]` with `access`, `description`, `technical_sheet`, `files` (its own `[PlaceFile]`, not the system's) and `points` (`[PlacePoint]`) |

### `PlaceFile`

| Field | Type |
|---|---|
| `id` | number |
| `place_id` | number, for a file of a place |
| `system_id` | number, for a file of a system |
| `title` | text, absent if it has no caption |
| `file_name` | text |
| `content_type` | text |
| `file_size` | number (bytes) |
| `uploaded_by` | text (the name of the account) |
| `created_at` | datetime |

### `PlacePoint`

| Field | Type |
|---|---|
| `id` | integer |
| `place_id` | integer |
| `kind` | parking/approach/waypoint/access/other |
| `name` | text |
| `lat` | number |
| `lon` | number |
| `altitude_m` | integer |
| `note` | text |
| `created_at` | datetime |

### `OsmPoint`

What a map needs to draw an element of the mirror.

| Field | Type |
|---|---|
| `type` | node/way/relation |
| `id` | integer (unique only among those of its type) |
| `lat` | number (the node itself, or a point inside the shape of a way or a relation) |
| `lon` | number |
| `category` | cave/canyon/mine/hut/via_ferrata |
| `name` | text, left out if the element has none |
| `where` | text, "municipality, province" ("Rasines, Cantabria"); only a search (`/osm/search`) gives it, and it is left out if the element is in no known municipality |

### `OsmChange`

| Field | Type |
|---|---|
| `key` | text: the OpenStreetMap tag (`name`, `name:es`, `alt_name`, `ele`, `description`, `website`, `access`, `fee`, `capacity`, `beds`, `operator`, `max:abseil`, `rating`, `depth`, `length`, or what says what the element is), or `position` (the point moved more than 10 m), or `removed` (the element is no longer in OpenStreetMap) |
| `before` | text, left out if the tag was not there |
| `after` | text, left out if the tag is not there any more |

### `OsmCandidate`

An `OsmPoint` and:

| Field | Type |
|---|---|
| `where` | text, the municipality and the province of the element ("Rasines, Cantabria"); left out if it is in no known zone |
| `distance_m` | integer, metres from the point asked for |
| `name_match` | exact/similar, left out if the name does not look like the one asked |

### `OsmElement`

| Field | Type |
|---|---|
| `type`, `id`, `lat`, `lon`, `category` | as in `OsmPoint` |
| `version` | integer, of the element in OpenStreetMap |
| `timestamp` | datetime, when it was last edited there |
| `tags` | object of text, all the tags as they are |
| `geometry` | GeoJSON of its outline (a polygon, a line or a `MultiLineString` for a canyon), null for a node |
| `deleted_at` | datetime, when it stopped being in OpenStreetMap; null if it is |
| `area_id` | integer, the zone of `areas` the element is in; left out if it is in none known |
| `where` | text, "municipality, province", left out if the element is in no known municipality |
