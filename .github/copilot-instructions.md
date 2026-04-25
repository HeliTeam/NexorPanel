# Nexor VPN Panel — development guide

## Project overview

Nexor is a web control panel for **Xray-core**, forked from [3x-ui](https://github.com/MHSanaei/3x-ui). It adds subscription users, a `/api` surface (JWT + API keys), hardened defaults, and Nexor-branded paths (`/etc/nexor`, etc.). Stack: Go, Gin, embedded assets, GORM (SQLite or Postgres via DSN).

## Architecture (high level)

- **main.go** — DB init, web + sub server, graceful shutdown
- **web/** — Gin router, HTML templates, static assets (`//go:embed`)
- **xray/** — Xray process management and API
- **database/** — models and migrations (`database/model/`)
- **sub/** — subscription HTTP server
- **web/service/** — business logic (inbounds, Nexor users, settings, bot, …)
- **web/controller/** — HTTP handlers
- **web/job/** — cron jobs (traffic, expiry, IP checks, …)

## Embedded resources

`web/assets`, `web/html`, `web/translation` are embedded at compile time. With `XUI_DEBUG=true`, templates/assets can be served from disk for faster UI iteration.

## Building and running

```bash
go build -o nexor .
XUI_DEBUG=true go run ./main.go
go test ./...
```

VS Code tasks build to `bin/nexor.exe` (Windows).

## Configuration

- Env: `XUI_DEBUG`, `XUI_LOG_LEVEL`, `XUI_MAIN_FOLDER`, database DSN as configured in the app
- Product name/version: `config/name`, `config/version`

## Tests and paths

Integration tests may use a temporary DB file; panel production paths are documented in `README.md` and `deploy/`.

## Upstream

Protocol and Xray concepts: [3x-ui Wiki](https://github.com/MHSanaei/3x-ui/wiki). Release tarballs from this repo may still use an `x-ui` directory layout for installer compatibility.
