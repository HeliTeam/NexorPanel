[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md)

<p align="center">
  <img alt="Nexor" src="./media/logo_nexor.png" width="160" height="160">
</p>

# Nexor VPN Panel

**Nexor** is a commercial-oriented web control panel for **Xray-core**, based on the [3x-ui](https://github.com/MHSanaei/3x-ui) codebase. It adds a Nexor domain layer (subscription users, REST API with JWT/API keys, hardened defaults, and rebranded paths).

[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)
[![Go module](https://img.shields.io/badge/go%20module-github.com%2Fnexor%2Fpanel-00ADD8)](https://github.com/HeliTeam/NexorPanel)

> [!IMPORTANT]
> Use only where legally permitted. This fork is intended for authorized VPN / proxy operations.

## Quick start (from source)

```bash
git clone https://github.com/HeliTeam/NexorPanel.git && cd NexorPanel
go build -o nexor .
sudo mkdir -p /usr/local/nexor /etc/nexor /var/log/nexor
sudo cp nexor /usr/local/nexor/
sudo cp deploy/nexor.service /etc/systemd/system/nexor.service
sudo systemctl daemon-reload
# First admin (interactive):
sudo /usr/local/nexor/nexor create-admin
sudo systemctl enable --now nexor
```

Upstream **one-click** installers target the original `x-ui` binary layout; for Nexor production use the binary you build and the unit file under [`deploy/nexor.service`](deploy/nexor.service).

Module path for imports: `github.com/nexor/panel`.

## Documentation

- Upstream wiki (protocol / Xray concepts): [3x-ui Wiki](https://github.com/MHSanaei/3x-ui/wiki)

## A Special Thanks to

- [alireza0](https://github.com/alireza0/) and [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) contributors

## Acknowledgment

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (License: **GPL-3.0**)
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (License: **GPL-3.0**)

## Support project

If this project is helpful to you, you may wish to give it a star on GitHub.
