# CLAUDE.md

## Project Overview

Hub is a team management platform for the Triax American football team (members, events, equipment, player numbers). Go backend + Vite/React/TanStack Router frontend monorepo deployed to Google App Engine. The project is Japanese — comments, UI text, commit messages, and cron descriptions are often in Japanese.

## Commands

```bash
# Install
npm install
go mod download

# Dev
npm run dev          # Vite frontend dev server (port 3000, proxies API to :8080)
go run main.go       # Go API server (port 8080)

# Build
npm run build        # Production build (output: client/dest/)
go build -v ./...

# Test
npm run test         # Vitest (frontend)
go test -v ./...     # Go (backend)

# Lint
npm run lint         # ESLint
```

## Architecture — Key Decisions

- **No Node.js server in production.** Vite builds a static SPA. The Go server serves `index.html` for all page routes and handles auth + API.
- **SPA with TanStack Router.** Client-side routing is handled by TanStack Router. Go auth middleware checks authentication on initial page load; subsequent navigation is client-side.
- **Use native `<img>` tags, NOT framework image components.**
- All routes are defined in `main.go` — API under `/api/1/*` (JSON, JWT auth), pages under `/*` (serve SPA index.html via marmoset), cron tasks under `/tasks/*` (GAE admin-only). Static assets under `/assets/*`.
- Frontend routes are defined in `client/src/router.tsx` using TanStack Router.
- Frontend API calls go through the repository pattern in `client/repository/`.
- Go API handlers return JSON via `marmoset.Render().JSON()`. Page controllers serve `index.html` via marmoset templates.
- Database is Google Cloud Datastore (NoSQL). Models are in `server/models/`.
- Environment variables: use `import.meta.env.VITE_*` for frontend (e.g., `VITE_API_BASE_URL`).

## Code Style

- Frontend: 2-space indentation (ESLint enforced), Tailwind CSS for styling
- Backend: standard `gofmt` formatting
- Follow existing patterns — see `client/src/pages/` for page examples, `server/api/` for endpoint examples

## Git Workflow

- `main` — production (auto-deploys to GAE on merge)
- `develop` — staging (PRs from develop to main trigger dev deploy)
- Feature branches → PR to `develop` or `main`

## Local Dev Setup

Requires `secrets.local.yaml` — copy from `secrets.example.yaml` and fill in real values. See that file for all required env vars (Slack tokens, Google credentials, JWT keys, etc.). Never commit secret files.
