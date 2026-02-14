# CLAUDE.md

## Project Overview

Hub is a team management platform for the Triax American football team (members, events, equipment, player numbers). Go backend + Next.js frontend monorepo deployed to Google App Engine. The project is Japanese — comments, UI text, commit messages, and cron descriptions are often in Japanese.

## Commands

```bash
# Install
npm install
go mod download

# Dev
npm run dev          # Next.js frontend dev server (port 3000)
go run main.go       # Go API server (port 8080)
npx too              # Full local dev: Datastore emulator + Go + Next.js

# Build
npm run build        # Dev build (output: client/dest/)
npm run export       # Production static export (NODE_ENV=production)
go build -v ./...

# Test
npm run test         # Jest (frontend)
go test -v ./...     # Go (backend)

# Lint
npm run lint         # ESLint via next lint
```

## Architecture — Key Decisions

- **No Node.js server in production.** Next.js runs in static export mode. The Go server serves the exported HTML/JS and handles all routing and auth.
- **IMPORTANT: Use native `<a>` tags, NOT Next.js `<Link>`.** This is intentional — every navigation must hit the Go server for authentication. The ESLint rule `no-html-link-for-pages` is disabled for this reason.
- **Use native `<img>` tags, NOT Next.js `<Image>`.** The `no-img-element` rule is disabled.
- All routes are defined in `main.go` — API under `/api/1/*` (JSON, JWT auth), pages under `/*` (HTML via marmoset templates), cron tasks under `/tasks/*` (GAE admin-only).
- Frontend API calls go through the repository pattern in `client/repository/`.
- Go API handlers return JSON via `marmoset.Render().JSON()`. Controllers render HTML via marmoset templates.
- Database is Google Cloud Datastore (NoSQL). Models are in `server/models/`.

## Code Style

- Frontend: 2-space indentation (ESLint enforced), Tailwind CSS for styling
- Backend: standard `gofmt` formatting
- Follow existing patterns — see `client/pages/` for page examples, `server/api/` for endpoint examples

## Git Workflow

- `main` — production (auto-deploys to GAE on merge)
- `develop` — staging (PRs from develop to main trigger dev deploy)
- Feature branches → PR to `develop` or `main`

## Local Dev Setup

Requires `secrets.local.yaml` — copy from `secrets.example.yaml` and fill in real values. See that file for all required env vars (Slack tokens, Google credentials, JWT keys, etc.). Never commit secret files.
