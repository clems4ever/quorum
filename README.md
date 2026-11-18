# Quorum

Find the time that works for everyone. Quorum is a small Doodle-style scheduler: sign in with Google, propose an event with candidate **days** or **hourly time slots**, share the link and let participants vote *yes* / *if need be* / *no*. The organizer can then close the poll and announce the final date.

- **Backend:** Go (standard library `net/http`, SQLite via the pure-Go `modernc.org/sqlite`, `golang.org/x/oauth2`)
- **Frontend:** React + [Mantine](https://mantine.dev) 9 (light and dark themes), Vite
- **Auth:** Google OpenID Connect (authorization code flow), server-side sessions in an HttpOnly cookie

## Features

- Create, edit, close/reopen and delete any number of events
- Two kinds of polls: whole days (calendar picker) or time slots (week grid with click-and-drag, configurable slot length and day bounds)
- Vote matrix with per-slot totals, the most popular option highlighted, and a final "chosen" date
- Dashboard listing the events you organize or have answered, with filters and search
- Optional cover image per event (JPEG, PNG, WebP or GIF up to 10 MB). Images are re-encoded on upload, which strips metadata such as GPS location and scales them to at most 1600 px. They are stored in SQLite next to the events. Organizers drag the image (or use the arrow keys) to choose its focal point, which every view keeps visible when cropping.
- Light/dark theme toggle (follows the OS by default)
- English and French: the language follows the browser, and a picker in the header stores an explicit choice. Dates, durations and API error messages are localized too.

## Translations

The UI uses [i18next](https://www.i18next.com/). Strings are in `web/src/i18n/en.ts` and `fr.ts`. A test checks that both files have the same keys. API errors include a stable `code` (e.g. `poll_closed`), which the client translates under `apiErrors.*`. The fake Google page picks its language from `Accept-Language`. To add a language, add a resource file, register it in `web/src/i18n/index.ts`, and import the matching `dayjs/locale/*`.

## Layout

```
cmd/quorum        production binary (cobra CLI: `quorum serve`)
cmd/fakegoogle    fake Google OAuth server for demos (separate binary)
internal/app      wiring and the `quorum` command
internal/envflag  flag → environment variable fallbacks
internal/httpserver  graceful HTTP serving
internal/api      JSON API (/api/*)
internal/auth     Google provider, sign-in handlers, sessions
internal/store    SQLite persistence
internal/static   SPA file server
internal/fakegoogle  fake provider + single-origin gateway
web/              React/Mantine client
```

Dependencies are injected (store, authenticator, clock, ID generator, HTTP client, OAuth provider). Production code has no test hooks. The fake Google provider is its own binary: the production server only differs by the OAuth endpoint URLs it is configured with.

## Configuration (`quorum serve`)

Every setting is a flag with an environment variable fallback. Flags win over the environment, and the environment wins over defaults. Pass secrets through the environment. Run `quorum serve --help` for the full list, and `quorum --version` for the build version.

| Flag | Environment | Default | Description |
|---|---|---|---|
| `--google-client-id` / `--google-client-secret` | `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | none (required) | OAuth client from the Google Cloud console |
| `--public-url` | `PUBLIC_URL` | `http://localhost:8080` | External URL. The redirect URI is `<public-url>/auth/callback`. Cookies are `Secure` when it is https |
| `--addr` | `ADDR` | `:8080` | Listen address |
| `--database` | `DATABASE_PATH` | `quorum.db` | SQLite file |
| `--static-dir` | `STATIC_DIR` | `web/dist` | Built web client |
| `--session-ttl` | `SESSION_TTL` | `720h` | Session lifetime |
| `--google-auth-url`, `--google-token-url`, `--google-userinfo-url` | `GOOGLE_AUTH_URL`, … | Google's endpoints | Override only to use another provider (e.g. the fake one) |

In the Google Cloud console, create an OAuth client of type *Web application* and add `https://<your-host>/auth/callback` as an authorized redirect URI.

## Development

```sh
# backend
golangci-lint run                  # lint (config: .golangci.yml, golangci-lint v2)
go test ./...                      # unit + end-to-end tests
go build -o bin/quorum ./cmd/quorum && go build -o bin/fakegoogle ./cmd/fakegoogle
bin/quorum serve --help

# frontend
cd web && npm ci
npm run lint                       # ESLint (typescript-eslint type-checked + React hooks)
npm run format                     # Prettier (CI runs format:check)
npm run coverage                   # vitest, fails below 85%
npm run build                      # type check + bundle

# run everything locally with fake Google on http://localhost:9090
STATIC_DIR=web/dist scripts/demo.sh
```

For frontend hot reload, run `scripts/demo.sh`, then `BACKEND_URL=http://localhost:9090 npm run dev` in `web/`.

## Demo mode (fake Google)

`cmd/fakegoogle` implements Google's authorize, token and userinfo endpoints. Its account chooser lists a few predefined profiles (and accepts a custom name and email): click one and you are signed in. `scripts/demo.sh` serves the app and the fake provider on a single origin. `/fake-google/*` goes to the fake provider and everything else is proxied to the app.

```sh
docker build --target demo -t quorum:demo .
docker run -p 9090:9090 -e PUBLIC_URL=http://localhost:9090 -v quorum-data:/data quorum:demo
```

## Production image

```sh
docker build -t quorum .          # default target: distroless, non-root
docker run -p 8080:8080 -v quorum-data:/data \
  -e PUBLIC_URL=https://quorum.example.com \
  -e GOOGLE_CLIENT_ID=... -e GOOGLE_CLIENT_SECRET=... quorum
```

## Database

SQLite with versioned migrations (`PRAGMA user_version`), applied automatically at startup. To change the schema, append a migration to `migrations` in `internal/store/store.go`; never edit a released one.

## Docker Compose with Traefik

`compose.yaml` runs the published image behind an existing [Traefik](https://traefik.io/) (v2 or v3, Docker provider), with HTTPS through a certificate resolver, HSTS, and Traefik health checks on `/healthz`. The container runs read-only, as non-root, with no capabilities. Data lives in the `quorum-data` volume.

```sh
cp .env.example .env    # set QUORUM_HOST, GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET
docker compose up -d
```

Traefik must share the external network `TRAEFIK_NETWORK` (default `traefik`) and provide the entrypoint `TRAEFIK_ENTRYPOINT` (default `websecure`) and the resolver `TRAEFIK_CERTRESOLVER` (default `letsencrypt`). Add `https://<QUORUM_HOST>/auth/callback` to the OAuth client's redirect URIs. Pin a build with `QUORUM_TAG=sha-<commit>`.

`compose.demo.yaml` builds and runs the demo image with the fake Google sign-in on `QUORUM_DEMO_HOST`: `docker compose -f compose.demo.yaml up -d --build`. Don't use it for real data, because anyone can sign in as anybody.

## CI

`.github/workflows/ci.yml` runs on pull requests and on every push to `main`:

1. **Backend:** golangci-lint, `go build`, `go test -race`. The job fails if total coverage is below 85%.
2. **Frontend:** Prettier check, ESLint (no warnings allowed), type check and production build, plus vitest with 85% coverage thresholds.
3. **Docker:** both images (production and demo) are built on every run. On `main` only, the production image is published as `ghcr.io/<owner>/<repo>:latest` and `:sha-<commit>`, with the commit as `quorum --version`. The demo image is never published; build it locally (`docker build --target demo .`).
