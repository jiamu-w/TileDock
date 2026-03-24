# TileDock

[中文](./README_CH.md) | [English](./README.md)

TileDock is a lightweight self-hosted navigation panel built with Go 1.22, Gin, GORM, SQLite, and `html/template`, with a focus on drag-and-tile layout editing.

The current version uses:

- Go server-side rendering
- A small amount of vanilla JavaScript + `fetch`
- SQLite single-file database
- Session-based authentication
- `embed` for templates and static assets, with single-binary distribution support
- Docker deployment support

The project prioritizes maintainability, simple deployment, and a clear structure before adding more features.

## Features

- User login / logout
- Dashboard home page
- Navigation group management
- Navigation link CRUD
- Link icons support local upload and optional automatic website favicon fetching
- Drag sorting for groups and links
- Floating settings panel
- Local dashboard background upload with automatic WebP compression
- Automatic cleanup of old background files after replacement
- Adjustable background blur and dark overlay opacity
- Chrome / Firefox bookmark HTML import
- Audit logs for key administrative actions
- `/healthz` health check
- Unified error handling
- Basic unit tests

## Tech Stack

- Go 1.22+
- Gin
- GORM
- SQLite
- `html/template`
- Vanilla JavaScript
- `log/slog`

## Project Structure

```text
.
├── cmd/server/main.go
├── config/config.yaml
├── internal
│   ├── config
│   ├── handler
│   ├── middleware
│   ├── model
│   ├── repository
│   ├── router
│   ├── service
│   └── view
├── pkg
│   ├── db
│   └── logger
├── static
│   ├── css
│   ├── js
│   └── uploads
├── templates
│   ├── auth
│   ├── dashboard
│   └── partials
├── Dockerfile
├── Makefile
├── README.md
└── README.en.md
```

## First Initialization

This version no longer uses insecure default administrator credentials.

If the `users` table is empty on first startup, you must explicitly provide the admin username and password. Otherwise, the application will refuse to start.

Development example:

```bash
PANEL_DEFAULT_ADMIN_USER=admin \
PANEL_DEFAULT_ADMIN_PASSWORD='dev-password-123' \
make run
```

Production example:

```bash
PANEL_APP_ENV=production \
PANEL_SESSION_SECRET='replace-with-a-random-string-of-at-least-32-characters' \
PANEL_SESSION_SECURE=true \
PANEL_DEFAULT_ADMIN_USER=admin \
PANEL_DEFAULT_ADMIN_PASSWORD='replace-with-a-strong-password' \
./bin/tiledock
```

## Configuration

The application has built-in defaults, but security-sensitive settings do not use weak usable defaults.

If `config/config.yaml` exists, it overrides the built-in defaults. Environment variables have the highest priority.

Common environment variables:

```bash
export PANEL_CONFIG="config/config.yaml"
export PANEL_SERVER_ADDR=":8080"
export PANEL_DB_PATH="data/panel.db"
export PANEL_UPLOAD_DIR="data/uploads"
export PANEL_BACKUP_DIR="data/backups"
export PANEL_SESSION_NAME="panel_session"
export PANEL_SESSION_SECRET="replace-with-at-least-32-random-characters"
export PANEL_SESSION_MAX_AGE="604800"
export PANEL_SESSION_SECURE="false"
export PANEL_SESSION_HTTP_ONLY="true"
export PANEL_DEFAULT_ADMIN_USER="admin"
export PANEL_DEFAULT_ADMIN_PASSWORD="strong-password"
export PANEL_LOG_LEVEL="info"
export PANEL_APP_ENV="development"
```

## Local Development

Requirements:

- Go 1.22+
- CGO enabled

Install dependencies and start:

```bash
make tidy
PANEL_DEFAULT_ADMIN_USER=admin \
PANEL_DEFAULT_ADMIN_PASSWORD='dev-password-123' \
make run
```

Or run directly:

```bash
PANEL_DEFAULT_ADMIN_USER=admin \
PANEL_DEFAULT_ADMIN_PASSWORD='dev-password-123' \
go run ./cmd/server
```

Then open:

- Login: `http://localhost:8080/login`
- Dashboard: `http://localhost:8080/`
- Health check: `http://localhost:8080/healthz`

## Single-Binary Distribution

The current version supports single-binary distribution.

Build:

```bash
go build -o bin/tiledock ./cmd/server
```

For distribution, the minimum required artifact is:

- `tiledock` executable

At runtime, the app will automatically use these default writable paths:

- `data/panel.db`
- `data/uploads`
- `data/backups`

Optional:

- Provide `config/config.yaml` only if you want to override defaults
- Or set custom paths via environment variables

## Common Commands

```bash
make run
make test
make build
make tidy
```

## Docker

Build image:

```bash
docker build -t tiledock:latest .
```

Run container:

```bash
docker run --rm \
  -p 8080:8080 \
  -e PANEL_SESSION_SECRET="replace-with-at-least-32-random-characters" \
  -e PANEL_DEFAULT_ADMIN_USER="admin" \
  -e PANEL_DEFAULT_ADMIN_PASSWORD="strong-password" \
  -v "$(pwd)/data:/app/data" \
  tiledock:latest
```

Notes:

- SQLite database is written to `/app/data/panel.db`
- Uploaded background files are stored in `/app/data/uploads`
- Backup files are stored in `/app/data/backups`

## Dashboard Interaction Notes

- The dashboard is the primary page, and the sidebar has been removed
- Settings open from the top button as a floating panel instead of a separate management page
- In edit mode you can:
  - Create groups
  - Create links
  - Edit groups
  - Edit links
  - Delete groups and links
  - Drag to reorder
- Outside edit mode:
  - Clicking a link opens it in a new window
  - Editing actions are hidden or disabled

## Tests

Run all tests:

```bash
go test ./...
```

## License

MIT. See `LICENSE`.

## Deployment Notes

- In production, `PANEL_SESSION_SECRET` must be explicitly set and be at least 32 characters
- In production, `PANEL_SESSION_SECURE=true` is required
- On first startup, `PANEL_DEFAULT_ADMIN_USER` and `PANEL_DEFAULT_ADMIN_PASSWORD` must be explicitly provided
- Persist the `data` directory
- Use HTTPS even behind a reverse proxy
- Backup restore requires the current password and enforces zip upload limits
- The settings panel can import Chrome / Firefox bookmark HTML and convert it into groups and links
- Background images only accept locally uploaded `/static/uploads/backgrounds/...` paths
- Key actions write structured audit logs, including login, settings changes, navigation CRUD, backup export, and restore
