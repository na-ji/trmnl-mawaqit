# TRMNL Mawaqit Prayer Times Plugin

A [TRMNL](https://usetrmnl.com) plugin that displays Islamic prayer times on your e-ink device, sourced from [Mawaqit](https://mawaqit.net) mosque data.

The only configuration is your mosque slug (e.g. `tawba-bussy-saint-georges`), and the plugin renders prayer times for the current day with the next upcoming prayer highlighted in bold.

## Features

- Real-time prayer times from Mawaqit's mosque calendar data
- Next prayer automatically highlighted in bold
- Jumua (Friday prayer) times displayed in the footer
- 4 TRMNL layout variants: full, half horizontal, half vertical, quadrant
- Settings page for configuring your mosque
- 1-hour API response caching to minimize external calls

## Quick Start

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

### Run with Docker

```bash
docker build -t trmnl-mawaqit .
docker run -p 8080:8080 \
  -e TRMNL_CLIENT_ID=your_client_id \
  -e TRMNL_CLIENT_SECRET=your_client_secret \
  -e MAWAQIT_API_BASE=https://yourapi/api/v1 \
  -v mawaqit-data:/app/data \
  trmnl-mawaqit
```

## Environment Variables

| Variable              | Required | Default             | Description                                                                  |
|-----------------------|----------|---------------------|------------------------------------------------------------------------------|
| `TRMNL_CLIENT_ID`     | Yes      | -                   | OAuth client ID from TRMNL plugin registration                               |
| `TRMNL_CLIENT_SECRET` | Yes      | -                   | OAuth client secret                                                          |
| `MAWAQIT_API_BASE`    | Yes      | -                   | Unoficial Mawaqit API base URL (cf https://github.com/mrsofiane/mawaqit-api) |
| `PORT`                | No       | `8080`              | HTTP listen port                                                             |
| `DB_PATH`             | No       | `./data/mawaqit.db` | SQLite database file path                                                    |

## TRMNL Plugin Registration

When registering this plugin on the TRMNL developer portal, configure these endpoint URLs (replace `BASE_URL` with your public server URL):

| Setting                   | URL                           |
|---------------------------|-------------------------------|
| Installation URL          | `{BASE_URL}/install`          |
| Installation Callback URL | `{BASE_URL}/install/callback` |
| Plugin Markup URL         | `{BASE_URL}/markup`           |
| Plugin Management URL     | `{BASE_URL}/manage`           |
| Uninstallation URL        | `{BASE_URL}/uninstall`        |

## Architecture

### Project Structure

```
trmnl-mawaqit/
├── main.go           # Entry point, router, config loading
├── handlers.go       # HTTP handlers for all TRMNL endpoints
├── mawaqit.go        # Mawaqit API client with in-memory caching
├── markup.go         # Prayer time computation + template rendering
├── store.go          # SQLite user storage (CRUD)
├── templates/
│   ├── full.html              # Full-screen layout (800x480)
│   ├── half_horizontal.html   # Half horizontal (800x240)
│   ├── half_vertical.html     # Half vertical (400x480)
│   ├── quadrant.html          # Quadrant (400x240)
│   └── manage.html            # Settings form
├── Dockerfile
└── .env.example
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
  POST /markup ──► Fetch Mawaqit data ──► Compute today's times
                   ──► Determine next prayer ──► Render 4 HTML layouts
                   ──► Return JSON with all markup variants
        │
        ▼
  POST /uninstall ──► Delete user from DB
```

### Key Components

**`store.go`** -- SQLite storage using `modernc.org/sqlite` (pure Go, no CGO). Stores user UUID, access token, mosque slug, and IANA timezone. WAL mode enabled for concurrent reads.

**`mawaqit.go`** -- HTTP client for the Mawaqit API. Fetches mosque data by slug and caches responses in memory for 1 hour per mosque. The API returns a calendar structure: 12 months, each a map from day number (string) to an array of 6 prayer times.

**`markup.go`** -- Takes Mawaqit API data and the user's timezone, extracts today's prayer times from the calendar, determines which prayer is next by comparing each time against the current local time, and renders all 4 template variants. If all prayers have passed for the day, Fajr is marked as next.

**`handlers.go`** -- HTTP handlers implementing the TRMNL plugin contract. The markup endpoint returns a JSON object with 4 keys (`markup`, `markup_half_horizontal`, `markup_half_vertical`, `markup_quadrant`), each containing rendered HTML for the corresponding TRMNL display size.

### Finding Your Mosque Slug

1. Go to [mawaqit.net](https://mawaqit.net)
2. Search for your mosque
3. The slug is the last part of the URL: `mawaqit.net/en/<your-mosque-slug>`
4. Enter this slug in the plugin settings page
