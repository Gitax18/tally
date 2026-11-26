# Security policy

## Reporting a vulnerability

Please do not report security problems in public issues, discussions or pull requests.

Report them privately through GitHub instead: open the repository's **Security** tab and choose
**Report a vulnerability**, or go straight to
<https://github.com/Gitax18/tally/security/advisories/new>.

Please include:

- what an attacker could do, and what they need first (for example, network access only, or a
  dashboard session);
- the steps to reproduce it, or a proof of concept;
- the version or commit you tested.

You can expect an acknowledgement within a few days. Once the problem is confirmed, a fix is
prepared privately and released, and the advisory is published with credit to you unless you
prefer otherwise.

## Supported versions

tally is young: fixes go into the latest release on `main` only.

## Scope

In scope: the Go service (tracker, event endpoint, `/v1` API), the dashboard in `web/`, and the
deployment files in this repository.

Out of scope:

- **Forged analytics events.** The event endpoint has to be public, so anyone can send made-up
  pageviews for a site whose ID they know. The hostname check, bot filter and rate limit only
  limit this; see "Known limitations" in the [README](README.md#known-limitations).
- Problems in Heroku, Vercel, Postgres or other third-party services themselves.
- Denial of service through sheer request volume.

For how tally handles visitor data, see [docs/privacy.md](docs/privacy.md).
