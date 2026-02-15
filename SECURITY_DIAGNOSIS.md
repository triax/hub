# Security Diagnosis Report

**Repository:** triax/hub
**Date:** 2026-02-15
**Scope:** Full codebase security audit (Go backend, React frontend, infrastructure, dependencies)

---

## Executive Summary

This report covers a comprehensive security audit of the Hub team management platform. The application is a Go + React monorepo deployed to Google App Engine, using Slack OAuth for authentication and Google Cloud Datastore for persistence.

**27 findings** were identified across 4 severity levels:

| Severity | Count |
|----------|-------|
| CRITICAL | 5 |
| HIGH | 9 |
| MEDIUM | 9 |
| LOW | 4 |

---

## CRITICAL Findings

### C-01: Hardcoded OAuth State and Nonce (CSRF/Replay)

**File:** `server/controllers/oauth.go:19-22`
```go
const (
    nonce = "xxxyyyzzz" // TODO: Fix
    state = "temp"      // TODO: Fix
)
```

**Impact:** The OAuth `state` parameter is meant to prevent CSRF attacks during the OAuth flow, and `nonce` is meant to prevent replay attacks. Both are hardcoded to static placeholder values, rendering them useless.

**Attack:** An attacker can craft a malicious link that initiates the OAuth flow and redirect the victim to an attacker-controlled callback, potentially hijacking the session.

**Fix:** Generate cryptographically random `state` and `nonce` per request using `crypto/rand`, store in a short-lived server-side store or signed cookie, and validate on callback.

---

### C-02: Session Cookie Missing Security Flags

**File:** `server/controllers/oauth.go:166-171`
```go
http.SetCookie(w, &http.Cookie{
    Name:    server.SessionCookieName,
    Value:   tokenstr,
    Path:    "/",
    Expires: time.Now().Add(server.ServerSessionExpire),
    // Missing: HttpOnly, Secure, SameSite
})
```

**Impact:** The session JWT cookie is created without:
- `HttpOnly` -- cookie is accessible to JavaScript (XSS can steal sessions)
- `Secure` -- cookie is transmitted over plain HTTP
- `SameSite` -- no CSRF protection at the cookie level

**Fix:**
```go
http.SetCookie(w, &http.Cookie{
    Name:     server.SessionCookieName,
    Value:    tokenstr,
    Path:     "/",
    Expires:  time.Now().Add(server.ServerSessionExpire),
    HttpOnly: true,
    Secure:   true,
    SameSite: http.SameSiteLaxMode,
})
```

---

### C-03: ReDoS via Unescaped User Input in Regex

**File:** `server/api/members.go:36-37`
```go
if w := req.URL.Query().Get("keyword"); w != "" {
    exp, err := regexp.Compile("(?i)" + w)
```

**Impact:** The `keyword` query parameter is concatenated directly into a regex pattern. An attacker can submit a malicious pattern like `(a+)+b` to cause catastrophic backtracking, consuming CPU and potentially causing a denial of service.

**Fix:** Use `regexp.QuoteMeta(w)` to escape all regex metacharacters before compilation:
```go
exp, err := regexp.Compile("(?i)" + regexp.QuoteMeta(w))
```

---

### C-04: Open Redirect in OAuth Callback

**File:** `server/controllers/oauth.go:173-174`
```go
if destination != "" {
    http.Redirect(w, req, destination, http.StatusTemporaryRedirect)
}
```

**Impact:** The `goto` query parameter is used as a redirect destination after authentication with no validation. An attacker can craft a login URL like `/login?goto=https://evil.com` to redirect the user to a malicious site after they authenticate.

**Fix:** Validate that the destination is a relative path on the same host:
```go
if destination != "" {
    u, err := url.Parse(destination)
    if err == nil && u.Host == "" && strings.HasPrefix(u.Path, "/") {
        http.Redirect(w, req, destination, http.StatusTemporaryRedirect)
        return
    }
}
http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
```

---

### C-05: No Authorization Checks on API Endpoints (Horizontal Privilege Escalation)

**Files:** All handlers in `server/api/`

**Impact:** While the auth middleware verifies JWT tokens (i.e., user is authenticated), **no handler checks whether the authenticated user is authorized** to perform the specific action. Examples:
- `POST /api/1/members/{id}/props` -- any authenticated user can modify ANY member's properties (`server/api/members.go:83`)
- `POST /api/1/events/{id}/delete` -- any user can delete ANY event (`server/api/events.go`)
- `POST /api/1/equips/{id}/delete` -- any user can delete ANY equipment (`server/api/equips.go`)
- `POST /api/1/numbers/{num}/assign` -- any user can assign player numbers (`server/api/numbers.go`)

**Fix:** Implement role-based access control. At minimum, verify that the requesting user's Slack ID matches the target resource owner, or check for an admin role before destructive operations.

---

## HIGH Findings

### H-01: JSON Injection in Slack Response URLs

**Files:**
- `server/slackbot/shortcuts.go:89-91`
- `server/slackbot/shortcuts.go:115-117`
- `server/slackbot/slashcommands.go:63`

```go
// shortcuts.go:89
http.Post(payload.ResponseURL, "application/json", strings.NewReader(
    fmt.Sprintf(`{"text":":white_check_mark: %s => %s"}`, payload.Message.Text, member.Name()),
))

// shortcuts.go:115 (Translate)
fmt.Sprintf(`{"text": "%s"}`, text)

// slashcommands.go:63
strings.NewReader(`{"text":"`+feedback+`"}`)
```

**Impact:** User-supplied text is interpolated directly into JSON strings without escaping. If the text contains `"`, `\`, or newlines, it breaks the JSON structure. This could allow injection of arbitrary JSON fields or cause requests to fail.

**Fix:** Use `json.Marshal()` to build the JSON payload properly:
```go
body, _ := json.Marshal(map[string]string{"text": text})
http.Post(url, "application/json", bytes.NewReader(body))
```

---

### H-02: Weak Slack Webhook Verification (Deprecated Token Method)

**Files:**
- `server/slackbot/slackevents.go:76`
- `server/slackbot/shortcuts.go:31`

```go
if payload.Token != bot.VerificationToken {
```

**Impact:** The application uses Slack's deprecated verification token instead of the modern HMAC-SHA256 request signing (`X-Slack-Signature`). The verification token is a static shared secret that provides weaker security guarantees.

**Fix:** Implement Slack's [request signing verification](https://api.slack.com/authentication/verifying-requests-from-slack) using the signing secret, `X-Slack-Request-Timestamp`, and `X-Slack-Signature` headers.

---

### H-03: Environment Variable Leak via Slack Bot

**File:** `server/slackbot/slackevents.go:116-127, 366-372`

```go
case "HUB_WEBPAGE_BASE_URL", "HUB_CONDITIONING_CHECK_SHEET_URL":
    bot.onEnvDump(req, w, event)

func (bot Bot) onEnvDump(_ *http.Request, _ http.ResponseWriter, event slackevents.AppMentionEvent) {
    name := largo.Tokenize(event.Text)[1:][0]
    bot.SlackAPI.PostMessage(event.Channel,
        slack.MsgOptionText("`"+os.Getenv(name)+"`", false),
    )
}
```

**Impact:** Although the switch statement limits this to two specific env var names, `onEnvDump` re-parses the event text independently and reads whatever environment variable name appears in the first token. This is a risky pattern -- if the tokenizer behaves differently than the switch, arbitrary environment variables could be leaked to a Slack channel.

**Fix:** Pass the matched variable name from the switch case instead of re-parsing:
```go
case "HUB_WEBPAGE_BASE_URL":
    bot.onEnvDump(req, w, event, "HUB_WEBPAGE_BASE_URL")
```

---

### H-04: Unmaintained JWT Library (golang-jwt v3)

**File:** `go.mod`
```
github.com/golang-jwt/jwt v3.2.2
```

**Impact:** The `golang-jwt/jwt` v3 line is **unmaintained**. The active branch is v5.x. CVE-2024-51744 affects versions prior to v4.5.1 where `ParseWithClaims` can return `nil` token with `nil` error for certain invalid tokens, potentially bypassing authentication.

**Fix:** Upgrade to `github.com/golang-jwt/jwt/v5` (latest stable). This requires updating import paths and adapting to the v5 API.

---

### H-05: End-of-Life Go Runtime

**Files:** `go.mod` (declares `go 1.18`), `app.yaml` (uses `runtime: go121`)

**Impact:** Go 1.18 reached EOL in February 2023. Go 1.21 reached EOL in August 2024. Both no longer receive security patches from the Go team. Running an EOL runtime in production means known vulnerabilities remain unpatched.

**Fix:** Update `go.mod` to `go 1.23` (or later) and `app.yaml` to `runtime: go123`.

---

### H-06: Vulnerable Go Dependencies

| Package | Current | CVE | Fix |
|---------|---------|-----|-----|
| `golang.org/x/crypto` | v0.25.0 | CVE-2024-45337 (CVSS 9.1) | >= v0.31.0 |
| `golang.org/x/net` | v0.27.0 | CVE-2024-45338 (HTML DoS) | >= v0.33.0 |
| `google.golang.org/grpc` | v1.64.1 | DoS vectors | >= v1.66.0 |

**Fix:** Run `go get -u golang.org/x/crypto golang.org/x/net google.golang.org/grpc && go mod tidy`.

---

### H-07: No Security Headers

**File:** `main.go` (no security header middleware)

**Impact:** The application sets **no security headers** on any response:
- No `Content-Security-Policy` -- allows loading scripts/styles from any origin
- No `X-Frame-Options` -- vulnerable to clickjacking
- No `X-Content-Type-Options` -- allows MIME type sniffing
- No `Strict-Transport-Security` -- no HSTS enforcement
- No `Referrer-Policy` -- full URL leaked in Referer headers

**Fix:** Add a global middleware to set security headers on all responses:
```go
func SecurityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' https://*.slack-edge.com https://*.slack.com; style-src 'self' 'unsafe-inline'")
        next.ServeHTTP(w, r)
    })
}
```

---

### H-08: No JWT Algorithm Validation

**File:** `server/controllers/oauth.go:150`
```go
t := jwt.New(jwt.GetSigningMethod(os.Getenv("JWT_SIGNING_METHOD")))
```

**File:** `server/filters/auth.go:76`
```go
jwt.ParseWithClaims(cookie.Value, claims, func(t *jwt.Token) (interface{}, error) {
    return []byte(os.Getenv("JWT_SIGNING_KEY")), nil
})
```

**Impact:** The key function in `ParseWithClaims` does not validate `t.Method` against an expected algorithm. An attacker could craft a JWT using the `none` algorithm or switch from HMAC to RSA (algorithm confusion), potentially bypassing signature verification.

**Fix:** Validate the signing algorithm in the key function:
```go
func(t *jwt.Token) (interface{}, error) {
    if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
    }
    return []byte(os.Getenv("JWT_SIGNING_KEY")), nil
}
```

---

### H-09: NPM Dependency Vulnerabilities

| Package | Severity | Issue |
|---------|----------|-------|
| `glob` (via sucrase) | HIGH | Command injection via `-c/--cmd` (GHSA-5j98-mcp5-4vw2) |
| `brace-expansion` (via sucrase) | LOW | ReDoS (GHSA-v6h2-p8h4-qcjw) |

**Fix:** Run `npm audit fix`.

---

## MEDIUM Findings

### M-01: No CSRF Token Validation on State-Changing Operations

**Files:** All POST API endpoints

**Impact:** POST requests rely solely on cookie-based authentication with no CSRF token. Combined with the missing `SameSite` cookie attribute (C-02), cross-site form submissions could perform actions on behalf of authenticated users.

**Fix:** Add `SameSite=Lax` to session cookies (minimum). For stronger protection, implement a CSRF token pattern.

---

### M-02: Cron Task Endpoints Lack In-Code Authentication

**File:** `main.go:100-109`

**Impact:** The `/tasks/*` routes rely entirely on GAE's `login: admin` directive in `app.yaml` for protection. The Go code itself performs no authentication checks. If the app.yaml configuration is misconfigured or the app is run outside GAE, all cron endpoints are publicly accessible and can manipulate the database.

**Fix:** Add middleware to verify the `X-Appengine-Cron: true` header in cron handlers:
```go
func RequireGAECron(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Header.Get("X-Appengine-Cron") != "true" {
            http.Error(w, "Forbidden", http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

---

### M-03: Error Messages Leak Internal Details

**Files:** Throughout all handlers

Examples:
- `server/controllers/oauth.go:93` -- `w.Write([]byte(err.Error()))` exposes raw HTTP client errors
- `server/controllers/oauth.go:85` -- internal error passed to client in redirect URL
- `server/api/members.go:21` -- `marmoset.P{"error": err.Error()}` exposes Datastore errors

**Impact:** Internal error details (stack traces, Datastore errors, HTTP client errors) are returned to clients, leaking implementation details that aid attackers.

**Fix:** Return generic error messages to clients and log detailed errors server-side.

---

### M-04: Debug Output in Production

**File:** `server/controllers/oauth.go:164`
```go
fmt.Printf("[DEBUG] %s = length(%d)\n", member.Slack.RealName, len(tokenstr))
```

Also: `server/slackbot/slashcommands.go:34-35`
```go
fmt.Println(1000)
fmt.Println(ids)
```

**Impact:** Debug statements in production code leak user names, token lengths, and internal data to stdout/logs.

**Fix:** Remove all debug `fmt.Print` statements. Use a structured logger with configurable log levels.

---

### M-05: No Rate Limiting

**Impact:** No rate limiting on any endpoint. Attackers can:
- Brute-force enumerate all members/events/equipment
- Cause resource exhaustion through repeated API calls
- Abuse the OAuth login flow

**Fix:** Implement per-IP or per-user rate limiting middleware, especially on `/login`, `/auth/*`, and API endpoints.

---

### M-06: No Input Length Validation

**Files:** All API handlers accepting request bodies

**Impact:** No maximum length validation on JSON request bodies or query parameters. Attackers can submit extremely large payloads to exhaust memory.

**Fix:** Use `http.MaxBytesReader` to limit request body sizes:
```go
req.Body = http.MaxBytesReader(w, req.Body, 1<<20) // 1MB max
```

---

### M-07: Frontend Missing HTTP Status Validation on Fetch Calls

**Files:** All repository files (`client/repository/*.ts`)

```typescript
// MemberRepo.ts
return fetch(endpoint).then(res => res.json()).then(Member.fromAPIResponse);
```

**Impact:** Fetch API does not reject on HTTP errors (4xx, 5xx). A 401 response is parsed as JSON and processed as valid data, potentially causing silent failures or broken UI state.

**Fix:** Check `res.ok` before parsing:
```typescript
fetch(endpoint).then(res => {
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
})
```

---

### M-08: Frontend POST Requests Missing Content-Type Header

**Files:** `client/repository/EquipRepo.ts`, `MemberRepo.ts`, `EventRepo.ts`

```typescript
return fetch(endpoint, {
  method: "POST",
  body: JSON.stringify(draft),
  // Missing: headers: { "Content-Type": "application/json" }
})
```

**Impact:** The server may misinterpret the content type. Some CORS configurations treat requests without an explicit `Content-Type: application/json` differently.

**Fix:** Add `headers: { "Content-Type": "application/json" }` to all POST requests.

---

### M-09: Secrets Written to Filesystem in CI/CD

**File:** `.github/workflows/gae-deploy.yml`

```yaml
- name: Recover App Secrets DEV
  run: 'echo "$APP_SECRETS_DEV_YAML" > secrets.dev.yaml'
  env:
    APP_SECRETS_DEV_YAML: ${{ secrets.APP_SECRETS_DEV_YAML }}
```

**Impact:** Secrets are written to the runner's filesystem as plaintext files. If the workflow fails or the runner is compromised, secrets could be exposed.

**Fix:** Use GCP Secret Manager for runtime secrets, or ensure secret files are explicitly deleted after deployment.

---

## LOW Findings

### L-01: CORS Wildcard in Local Development

**File:** `server/filters/auth.go:44`
```go
w.Header().Set("Access-Control-Allow-Origin", "*")
```

**Impact:** The wildcard CORS origin in local dev mode is overly permissive. While gated behind `LocalDev`, if the flag is misconfigured, any origin can access the API.

---

### L-02: 14-Day Session Duration

**File:** `server/const.go`
```go
ServerSessionExpire = time.Hour * 24 * 14
```

**Impact:** Sessions are valid for 14 days with no mechanism for revocation. If a token is stolen, it remains valid for up to 2 weeks.

---

### L-03: Hardcoded External Resource URLs

**Files:** `client/src/pages/errors.tsx`, `client/components/layout.tsx`

**Impact:** Hardcoded external image URLs (Twitter profile images, S3 avatars) could become unavailable or compromised.

---

### L-04: Workflow Actions Not Pinned to SHA

**Files:** `.github/workflows/*.yml`

**Impact:** Actions are pinned to major versions (`@v4`, `@v5`) rather than commit SHAs. A compromised action could inject malicious code into CI/CD.

**Fix:** Pin actions to full commit SHAs for supply chain security.

---

## Summary of Recommendations (Priority Order)

### Immediate (Critical)
1. Generate random OAuth `state`/`nonce` per request (C-01)
2. Add `HttpOnly`, `Secure`, `SameSite` to session cookies (C-02)
3. Use `regexp.QuoteMeta()` on user-supplied regex input (C-03)
4. Validate redirect destination is same-origin (C-04)
5. Add authorization checks to all API endpoints (C-05)

### Urgent (High)
6. Use `json.Marshal()` for all Slack JSON payloads (H-01)
7. Implement Slack HMAC-SHA256 request signing (H-02)
8. Pass env var name from switch case, not user input (H-03)
9. Upgrade `golang-jwt/jwt` to v5 (H-04)
10. Upgrade Go runtime to 1.23+ (H-05)
11. Update vulnerable Go dependencies (H-06)
12. Add security headers middleware (H-07)
13. Validate JWT signing algorithm (H-08)
14. Run `npm audit fix` (H-09)

### Important (Medium)
15. Add `SameSite` cookie attribute for CSRF (M-01)
16. Validate `X-Appengine-Cron` header in task handlers (M-02)
17. Return generic error messages to clients (M-03)
18. Remove debug `fmt.Print` statements (M-04)
19. Implement rate limiting (M-05)
20. Limit request body sizes (M-06)
21. Add HTTP status validation in frontend fetch calls (M-07)
22. Add `Content-Type` header to frontend POST requests (M-08)
23. Clean up secrets in CI/CD workflows (M-09)

### Recommended (Low)
24. Restrict CORS origins in dev mode (L-01)
25. Consider shorter session duration + token rotation (L-02)
26. Self-host external assets (L-03)
27. Pin GitHub Actions to commit SHAs (L-04)
