# CLAUDE.md

> This file provides context for AI assistants working on this codebase.

## Project Overview

**Hub** is a full-stack team management platform for the Triax American football team. It handles member management, event scheduling/RSVP, equipment tracking, player number assignments, and integrates with Slack and Google Calendar. The project is Japanese — code comments, commit messages, cron descriptions, and UI text are often in Japanese.

- **Repository:** https://github.com/triax/hub
- **Author:** otiai10
- **License:** MIT
- **Deployment:** Google App Engine (GAE)
- **Database:** Google Cloud Datastore (NoSQL)

## Architecture

This is a **monorepo** with a Go backend and a Next.js (React/TypeScript) frontend:

```
hub/
├── main.go                  # Go entry point — chi router setup, all route definitions
├── server/                  # Go backend
│   ├── api/                 # REST API handlers (JSON responses)
│   ├── controllers/         # Page controllers (HTML rendering via marmoset templates)
│   ├── filters/             # Middleware (JWT auth)
│   ├── models/              # Data models (Datastore entities)
│   ├── slackbot/            # Slack bot webhooks, shortcuts, slash commands
│   └── tasks/               # Cron/scheduled task handlers
├── client/                  # Next.js frontend
│   ├── pages/               # Next.js pages (SSG, exported as static HTML)
│   ├── components/          # React components
│   ├── models/              # TypeScript type definitions
│   ├── repository/          # API client classes (data access layer)
│   └── styles/              # Global SCSS + Tailwind CSS
├── app.yaml                 # GAE production config
├── app.dev.yaml             # GAE dev/staging config
├── cron.yaml                # GAE scheduled tasks
├── too.local.yaml           # Local dev orchestration (too)
└── secrets.example.yaml     # Template for required env vars
```

### Key Architectural Decisions

- **No Node.js server in production.** Next.js is used in static export mode (`output: "export"`). The Go server serves the static HTML/JS files and handles all routing/auth. Native `<a>` tags are used instead of Next.js `<Link>` to ensure requests always hit the Go server for authentication.
- **API routes** are mounted under `/api/1/*` with JWT authentication middleware.
- **Page routes** use the same auth middleware but render HTML via Go templates (marmoset).
- **Slack bot** handles events at `/slack/events`, shortcuts at `/slack/shortcuts`, and slash commands at `/slack/slashcommands`.
- **Cron tasks** are mounted under `/tasks/*` and protected by GAE admin login.

## Tech Stack

### Backend (Go)
- **Go 1.22** (CI) / Go 1.21 (GAE runtime) / Go 1.18 (go.mod)
- **chi/v5** — HTTP router
- **cloud.google.com/go/datastore** — Database
- **golang-jwt/jwt** — Authentication
- **marmoset** — Template rendering
- **slack-go/slack** — Slack API client
- **openaigo** — OpenAI/ChatGPT integration
- **appyaml** — YAML config loading

### Frontend (TypeScript/React)
- **Next.js 14.1** with static export
- **React 18.3**
- **TypeScript 4.3**
- **Tailwind CSS 3.0** with `@tailwindcss/forms` plugin
- **Headless UI + Heroicons** — UI components
- **SCSS/Sass** — Additional styling

## Common Commands

### Install Dependencies
```bash
npm install          # Frontend dependencies
go get -v -t -d ./... # Backend dependencies (or go mod download)
```

### Development
```bash
npm run dev          # Next.js dev server (frontend only, port 3000)
go run main.go       # Go API server (backend only, port 8080)
```

For full local development with Datastore emulator, use `too`:
```bash
npx too              # Starts: Datastore emulator + Go server + Next.js dev server
```
Requires `secrets.local.yaml` (copy from `secrets.example.yaml`).

### Build
```bash
npm run build        # Next.js development build (output to client/dest/)
npm run export       # Next.js production build (NODE_ENV=production, static export)
go build -v ./...    # Go build
```

### Test
```bash
npm run test         # Frontend: Jest tests
npm test -- --coverage # With coverage
go test -v ./...     # Backend: Go tests
go test -coverprofile=coverage.go.txt ./... # With coverage
```

### Lint
```bash
npm run lint         # ESLint (next lint ./client)
```

## Code Conventions

### Frontend (TypeScript/React)
- **2-space indentation** (enforced by ESLint)
- ESLint extends `next/core-web-vitals`
- `@next/next/no-html-link-for-pages` is disabled — use native `<a>` tags instead of `<Link>` (intentional, see architecture notes above)
- `@next/next/no-img-element` is disabled — use native `<img>` tags
- Repository pattern for API calls (`client/repository/`)
- TypeScript models in `client/models/`

### Backend (Go)
- Standard Go formatting (`gofmt`)
- Models are Datastore entities with appropriate field tags
- API handlers return JSON via marmoset's `Render().JSON()`
- Controllers render HTML pages via marmoset templates

### General
- Comments and UI text are often in **Japanese**
- No tsconfig.json — TypeScript config is handled by Next.js defaults

## CI/CD

### GitHub Actions Workflows

| Workflow | Trigger | What it does |
|---|---|---|
| `go.yml` | Push/PR to main, develop | Go build + test |
| `js.yml` | Push to develop, PR to main/develop | npm install, lint, test, build |
| `coverage.yml` | Push to main, develop | Codecov upload (Go + JS) |
| `codeql-analysis.yml` | Push to main, PR, weekly | Security scanning (Go + JS) |
| `gae-deploy.yml` | Push to main, PR to main | Full deploy pipeline |

### Deployment Flow

1. **Dev deploy:** PRs from `develop` branch to `main` → deploy to GAE dev service
2. **Prod deploy:** Merge to `main` → deploy `app.yaml` + `cron.yaml` to GAE → Slack announcement

### Branch Strategy
- `main` — Production
- `develop` — Development/staging
- Feature branches → PR to `develop` or `main`

## Environment Variables

See `secrets.example.yaml` for the full list. Key groups:

- **Slack:** `SLACK_BOT_USER_OAUTH_TOKEN`, `SLACK_CLIENT_ID`, `SLACK_CLIENT_SECRET`, `SLACK_INSTALLED_TEAM_ID`, `SLACK_BOT_EVENTS_VERIFICATION_TOKEN`
- **Google Cloud:** `GOOGLE_SERVICE_ACCOUNT_JSON`, `GOOGLE_CALENDAR_ID`
- **Auth:** `JWT_SIGNING_METHOD`, `JWT_SIGNING_KEY`, `BROWSER_SESSION_KEY_ID`
- **External APIs:** `OPENAI_API_KEY`, `DEEPL_API_TOKEN`
- **App:** `APP_ICON_URL`, `HUB_WEBPAGE_BASE_URL`, `HUB_HELP_PAGE_URL`, `HUB_CONDITIONING_CHECK_SHEET_URL`

For local development, create `secrets.local.yaml` from `secrets.example.yaml`. Never commit secret files.

## API Routes Reference

All API routes are defined in `main.go` and require JWT auth:

- `GET/POST /api/1/members` — Member management
- `GET/POST /api/1/events` — Event management (RSVP via `/events/answer`)
- `GET/POST /api/1/equips` — Equipment tracking (custody via `/equips/custody`)
- `GET/POST /api/1/numbers` — Player number assignment

## Scheduled Tasks (cron.yaml)

All times in Asia/Tokyo:
- **04:00** — Sync Slack members and Google Calendar events
- **12:05** — Check RSVP status
- **18:10–18:20** — Final call notifications (staff, offense, defense)
- **17:00** — Equipment reminders (day before)
- **17:30 / 21:10** — Equipment return report reminders
- **Tuesday 12:00** — Scan for unreported equipment
