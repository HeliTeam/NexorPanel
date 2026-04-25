[English](README.md) | [Русский](README.ru_RU.md)

<p align="center">
  <img alt="Nexor" src="./media/logo_nexor.png" width="160" height="160">
</p>

# Nexor VPN Panel

**Nexor** is a web control panel for **Xray-core**, forked from [3x-ui](https://github.com/MHSanaei/3x-ui). This repository (**[HeliTeam/NexorPanel](https://github.com/HeliTeam/NexorPanel)**) adds a Nexor-specific layer: subscription users, REST `/api` with JWT and API keys, automation jobs, optional PostgreSQL, rebranded paths (`/etc/nexor`, `/var/log/nexor`), and a refreshed UI.

[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)
[![Repository](https://img.shields.io/badge/GitHub-HeliTeam%2FNexorPanel-181717?logo=github)](https://github.com/HeliTeam/NexorPanel)
[![Go module](https://img.shields.io/badge/module-github.com%2Fnexor%2Fpanel-00ADD8)](https://github.com/HeliTeam/NexorPanel)

> [!IMPORTANT]
> Use only where it is legal. The panel is meant for authorized VPN / proxy operations you are allowed to run.

## Features (summary)

- **Admins** — panel operators; bootstrap via `create-admin`.
- **Subscription users** — Nexor user model tied to Xray clients, traffic and expiry policies.
- **REST API** — JWT + refresh, API keys, rate limiting and brute-force protection on sensitive routes.
- **Automation** — cron-style jobs (traffic, expiry, inactivity, etc.).
- **Database** — SQLite by default; PostgreSQL via DSN (see `deploy/nexor.service` comment).
- **Deploy** — example `systemd` unit and nginx snippets under [`deploy/`](deploy/).

**Documentation languages:** English (this file) and Russian ([README.ru_RU.md](README.ru_RU.md)) only.

## Requirements

- **Go** — same major/minor as in [`go.mod`](go.mod) (toolchain may download a newer patch).
- **CGO** — enabled for the default SQLite build; on Linux you need a C compiler and `libsqlite3-dev` (or equivalent).
- **Root or dedicated user** — the sample unit runs as `root`; adjust for your security model.

## Quick start (build from source, Linux)

```bash
git clone https://github.com/HeliTeam/NexorPanel.git
cd NexorPanel

# Dependencies (Debian/Ubuntu example)
sudo apt-get update
sudo apt-get install -y build-essential pkg-config libsqlite3-dev

export CGO_ENABLED=1
go build -ldflags "-w -s" -o nexor .

sudo mkdir -p /usr/local/nexor /etc/nexor /var/log/nexor
sudo cp nexor /usr/local/nexor/
sudo cp deploy/nexor.service /etc/systemd/system/nexor.service
sudo systemctl daemon-reload

# First administrator (interactive) — run before or after enabling the service
sudo /usr/local/nexor/nexor create-admin

sudo systemctl enable --now nexor
sudo systemctl status nexor --no-pager
```

Open the panel in a browser: `http://YOUR_SERVER:PORT/` (port is configured in panel settings). Open the same port (and subscription port if used) in your firewall / security group.

Environment variables (see [`deploy/nexor.service`](deploy/nexor.service)):

- `NEXOR_DB_FOLDER` / `XUI_DB_FOLDER` — database directory (default layout uses `/etc/nexor`).
- `NEXOR_LOG_FOLDER` / `XUI_LOG_FOLDER` — log directory (e.g. `/var/log/nexor`).
- Optional: `NEXOR_DATABASE_URL` for PostgreSQL.

Upstream **one-click** installers target the original `x-ui` layout; for Nexor in production, use the binary you build and the provided `nexor.service`.

**Go import path** (module): `github.com/nexor/panel`.

## Extra docs

- Xray / protocol concepts (upstream): [3x-ui Wiki](https://github.com/MHSanaei/3x-ui/wiki)
- Reverse proxy example: [`deploy/nginx-xray.helitop.ru.conf`](deploy/nginx-xray.helitop.ru.conf)

## Special thanks

- **Seizure** and **Klieer** — thank you for your energy and support. ♥
- **[HeliTeam](https://github.com/HeliTeam)** — core help in building and shipping this project.

With gratitude to upstream: [alireza0](https://github.com/alireza0/) and [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) contributors.

## Third-party data (routing)

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (License: **GPL-3.0**)
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (License: **GPL-3.0**)

## Stargazers

[![Stargazers over time](https://starchart.cc/HeliTeam/NexorPanel.svg?variant=adaptive)](https://starchart.cc/HeliTeam/NexorPanel)

If Nexor is useful to you, consider starring the repo on GitHub.
