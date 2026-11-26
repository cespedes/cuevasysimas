# Roadmap

What is not done yet, with the order it is expected in. What the board decided about opening the
catalogue (the site, the domain, the first places, what the public sees, the legal notice,
registration with a phone verified through Telegram) is in the repository of the intranet it comes
from (`docs/roadmap.md`, "Decided by the board"); this page keeps what concerns this project.

## The cutover from the intranet

The catalogue is born inside the intranet of the club and has to leave it once, without two sources
of truth: edition of places frozen for a short time, the data copied (`pg_dump --data-only
--schema=catalogue` and the directory of the place files), the issuer registered with its key
(`issuer register`), the intranet forwarding its members' requests with signed assertions, and the
schema dropped from the intranet's database. Until then this repository is tried next to the
intranet with a copy of the real data.

## Public side

- **Systems, files and points apart from the place** are not in the public routes or pages yet: they
  need their own redaction before they can be opened, one by one.
- **Maps**: the pages link to OpenStreetMap and embed nothing. If a map is ever embedded, the terms
  of the tile providers (IGN, Esri, OpenStreetMap) for a public site have to be settled first, and
  the credits.
- **The legal data** (`PUBLIC_SITE_LEGAL_*`): who answers for the site is still to be given, and the
  standard legal notice and privacy policy revised by whoever answers; both assume no cookies and no
  registration, and must be revised when registration opens.
- **A checklist in testing**, with a copy of the real data, of what an anonymous reader can and
  cannot get, including the sources other than the page: the API, the sitemap, the files by id.

## Registration of people outside the club

People outside the club can register with an email and a phone verified through Telegram (other
methods later, if they can be used). It needs a bot of the project's own, accounts of people with
their own sessions (the accounts exist; the sign-in does not), a sign-in that does not say whether
an address is known, the limits against abuse, an account page (data, deletion, export) and what a
registered person sees that the anonymous does not. Open points: whether the phone is mandatory,
which bot verifies it, and when registration opens.

## Later

Other groups and per-person rules (the rules are a table of audiences and parts, and `Group(name)`
is already an audience), proposals from registered people into a queue that an admin approves, other
clients of the catalogue, and the documentation of the OpenStreetMap integration revised for the
project.
