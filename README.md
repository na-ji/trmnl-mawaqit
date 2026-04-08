[[Lire en Français](README.fr.md)]

# TRMNL Mawaqit Prayer Times Plugin

A [TRMNL](https://usetrmnl.com) plugin that displays Islamic prayer times on your e-ink device, sourced from [Mawaqit](https://mawaqit.net) mosque data.

The only configuration is your mosque slug (e.g. `tawba-bussy-saint-georges`), and the plugin renders prayer times for the current day with the next upcoming prayer highlighted in bold.

## Features

- Real-time prayer times from Mawaqit's mosque calendar data
- Next prayer automatically highlighted in bold
- Jumua (Friday prayer) times displayed in the footer
- 4 TRMNL layout variants: full, half horizontal, half vertical, quadrant
- Settings page with i18n support (English and French)
- Two-layer smart caching: prayer times cached until Isha, markup cached until next prayer
- Structured logging with zerolog (JSON or console output)

## Getting Started

1. [Install the plugin on your TRMNL](https://trmnl.com/plugin_settings/new?keyname=mawaqit)
2. Find your mosque slug on [mawaqit.net](https://mawaqit.net) — it's the last part of the URL: `mawaqit.net/en/<your-mosque-slug>`
3. Enter the slug in the plugin settings page

## Self-Hosting

If you want to run your own instance of the server instead of using the hosted one:

### Run with Docker Compose

```bash
cp .env.example .env
# Edit .env with your TRMNL credentials

docker compose up -d
```

This pulls the pre-built image from `ghcr.io/na-ji/trmnl-mawaqit:main` and starts the server alongside the Mawaqit API.

### Environment Variables

| Variable              | Required | Default             | Description                                                                  |
|-----------------------|----------|---------------------|------------------------------------------------------------------------------|
| `TRMNL_CLIENT_ID`     | Yes      | -                   | OAuth client ID from TRMNL plugin registration                               |
| `TRMNL_CLIENT_SECRET` | Yes      | -                   | OAuth client secret                                                          |
| `MAWAQIT_API_BASE`    | Yes      | -                   | Unoficial Mawaqit API base URL (cf https://github.com/mrsofiane/mawaqit-api) |
| `PORT`                | No       | `8080`              | HTTP listen port                                                             |
| `DB_PATH`             | No       | `./data/mawaqit.db` | SQLite database file path                                                    |
| `LOG_FORMAT`          | No       | `console`           | Log output format: `console` for dev, `json` for production                  |

### TRMNL Plugin Registration

When registering your own plugin on the TRMNL developer portal, configure these endpoint URLs (replace `BASE_URL` with your public server URL):

| Setting                   | URL                           |
|---------------------------|-------------------------------|
| Installation URL          | `{BASE_URL}/install`          |
| Installation Callback URL | `{BASE_URL}/install/callback` |
| Plugin Markup URL         | `{BASE_URL}/markup`           |
| Plugin Management URL     | `{BASE_URL}/manage`           |
| Uninstallation URL        | `{BASE_URL}/uninstall`        |

## Development

### Prerequisites

- Go 1.25+ (uses method-pattern routing)
- A TRMNL developer account with a registered plugin (provides `TRMNL_CLIENT_ID` and `TRMNL_CLIENT_SECRET`)

### Run Locally

```bash
cp .env.example .env
# Edit .env with your TRMNL credentials

# Set environment variables
export TRMNL_CLIENT_ID=your_client_id
export TRMNL_CLIENT_SECRET=your_client_secret
# Link to your unofficial Mawaqit API https://github.com/mrsofiane/mawaqit-api
export MAWAQIT_API_BASE=https://mawaqit.naj.ovh/api/v1

go run .
```

The server starts on port 8080 by default. Visit `http://localhost:8080/health` to verify.

### Preview Templates

Preview all 4 layout variants locally using [trmnlp](https://github.com/usetrmnl/trmnlp):

```bash
# Terminal 1: generate liquid files from Go templates (with auto-rebuild on changes)
go run . liquidgen --watch

# Terminal 2: start the trmnlp preview server
trmnlp serve
```

Or via Docker:
```bash
go run . liquidgen
docker run --publish 4567:4567 --volume "$(pwd):/plugin" trmnl/trmnlp serve
```

Go templates in `templates/` are the single source of truth. The `liquidgen` subcommand renders them with mock data into `src/*.liquid` files that trmnlp serves with the TRMNL Design System.

### Project Structure

```
trmnl-mawaqit/
├── main.go           # Entry point, router, config, logging middleware
├── handlers.go       # HTTP handlers for all TRMNL endpoints
├── mawaqit.go        # Mawaqit API client with Isha-based TTL caching
├── markup.go         # Prayer time computation, template rendering, markup cache
├── liquidgen.go      # CLI subcommand: generates src/*.liquid from Go templates
├── i18n.go           # Translations (EN/FR) and language detection
├── store.go          # SQLite user storage (CRUD)
├── cmd/healthcheck/  # Tiny health check binary for Docker HEALTHCHECK
├── templates/
│   ├── full.html              # Full-screen layout (800x480)
│   ├── half_horizontal.html   # Half horizontal (800x240)
│   ├── half_vertical.html     # Half vertical (400x480)
│   ├── quadrant.html          # Quadrant (400x240)
│   └── manage.html            # Settings form (i18n)
├── src/                       # Generated liquid files for trmnlp (gitignored)
├── .trmnlp.yml                # trmnlp config (file watching, timezone)
├── Dockerfile                 # Multi-stage build with distroless runner
├── docker-compose.yml
└── .github/workflows/
    └── docker.yml             # CI: build and push to GHCR
```

All Go code lives in `package main` -- no sub-packages needed at this scale.

### Plugin Lifecycle

The plugin implements the standard TRMNL plugin lifecycle:

```
User clicks "Install" on TRMNL
        │
        ▼
  GET /install ──► Exchange OAuth code for token ──► Redirect to TRMNL
        │
        ▼
  POST /install/callback ──► Store user (UUID, access token, timezone)
        │
        ▼
  GET /manage ──► Show mosque slug form
  POST /manage ──► Save mosque slug to DB
        │
        ▼
  POST /markup ──► Check markup cache ──► Fetch Mawaqit data
                   ──► Compute today's times ──► Determine next prayer
                   ──► Render 4 HTML layouts ──► Cache until next prayer
                   ──► Return JSON with all markup variants
        │
        ▼
  POST /uninstall ──► Delete user from DB
```

### Key Components

**`store.go`** -- SQLite storage using `modernc.org/sqlite` (pure Go, no CGO). Stores user UUID, access token, mosque slug, and IANA timezone. WAL mode enabled for concurrent reads.

**`mawaqit.go`** -- HTTP client for the Mawaqit API. Fetches mosque data by slug and caches responses until Isha time in the mosque's timezone. After the last prayer of the day, the cache expires so the next fetch retrieves tomorrow's data.

**`markup.go`** -- Takes Mawaqit API data and the user's timezone, extracts today's prayer times from the calendar, determines which prayer is next by comparing each time against the current local time, and renders all 4 template variants. If all prayers have passed for the day, Fajr is marked as next. Includes a per-user markup cache that expires at the next prayer time.

**`handlers.go`** -- HTTP handlers implementing the TRMNL plugin contract. The markup endpoint returns a JSON object with 4 keys (`markup`, `markup_half_horizontal`, `markup_half_vertical`, `markup_quadrant`), each containing rendered HTML for the corresponding TRMNL display size.

**`i18n.go`** -- Simple map-based translations for English and French. Language is auto-detected from the browser's `Accept-Language` header, with manual override via query parameter.

## Acknowledgments

- [I-vortex94](https://github.com/I-vortex94) and the [TRMNL](https://usetrmnl.com) team for creating the plugin templates
- [mrsofiane](https://github.com/mrsofiane) for the unofficial [Mawaqit API](https://github.com/mrsofiane/mawaqit-api)
