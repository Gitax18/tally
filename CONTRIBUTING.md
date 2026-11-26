# Contributing to tally

Thank you for taking the time. Bug reports, documentation fixes and code are all welcome.

## Before you start

- **Small fixes** (typos, clear bugs with an obvious fix): open a pull request directly.
- **Anything bigger** (a feature, a change to how a metric is counted, a new dependency): open an
  issue first and describe the problem you want to solve. Agreeing on the approach up front saves
  everyone a rewrite.
- **Security problems**: do not open an issue. See [SECURITY.md](SECURITY.md).

## Principles

tally is small on purpose. Every change has to keep these:

| Principle | In practice |
|---|---|
| Privacy is not a setting | No cookies or browser storage for visitors, no stored IP addresses or user agents, no fingerprinting, nothing that links a person across sites or days |
| The tracker stays tiny | `tracker/t.js` must stay under 1 KB gzipped once minified; a test enforces it |
| Boring infrastructure | Plain Postgres and one Go process. No new services, queues or caches without a very good reason |
| Every number is defined | A change to how something is counted needs tests with exact expected values and an update to [docs/metrics.md](docs/metrics.md) |
| The dashboard never touches the database | It talks to the service's `/v1` API only |

## Setting up

You need Go 1.27 or newer, Node 24 or newer with pnpm (`corepack enable`), Docker and `make`.
For linting, install [golangci-lint](https://golangci-lint.run) v2.

```sh
make db migrate seed      # Postgres, the schema, and fake data
make run                  # the service on :8090
cd web && pnpm install
cd .. && make web-dev     # the dashboard on :3010, password: tally-dev-password
```

## Making a change

1. Create a branch: `git checkout -b fix/short-description`.
2. Make the change, with tests:
   - Go code: a unit test next to it, or an integration test against Postgres when it touches SQL
     (see `internal/testdb`).
   - Dashboard logic: a unit test in `web/src/lib`.
   - Anything a visitor or the dashboard user would notice: consider the end-to-end test in
     `web/e2e`.
3. If you edited `tracker/t.js`, run `make tracker` and commit the regenerated `t.min.js`.
4. Run every check:

   ```sh
   make check                                           # go vet, golangci-lint, all Go tests
   cd web && pnpm lint && pnpm typecheck && pnpm test   # dashboard
   pnpm build && pnpm test:e2e                          # end to end
   ```

5. Commit with a message in the style of the history: `type: summary`, where `type` is one of
   `feat`, `fix`, `docs`, `test`, `refactor` or `chore`. Explain the why in the body.
6. Open a pull request. Describe what changed and why, and how you checked it. For dashboard
   changes, include screenshots in light and dark mode.

## Code style

- **Go**: `gofmt`, and golangci-lint with the repository's `.golangci.yml`. Errors are returned
  and wrapped, not logged and ignored. SQL uses bind parameters only.
- **TypeScript**: Prettier and ESLint as configured in `web/`. Server-only code imports
  `server-only`; the API secret never reaches a client component.
- **Comments** explain why, not what. Public behaviour that users or self-hosters see belongs in
  `docs/`.

## License

By contributing, you agree that your contributions are licensed under the [MIT License](LICENSE).
