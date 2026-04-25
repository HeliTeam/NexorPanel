<p align="center">
  <img alt="Nexor" src="./media/logo_nexor.png" width="160" height="160">
</p>

# Nexor VPN Panel

Веб-панель для **Xray-core**, развиваемая командой **[HeliTeam](https://github.com/HeliTeam)** в репозитории **[HeliTeam/NexorPanel](https://github.com/HeliTeam/NexorPanel)**. Кодовая база восходит к [3x-ui](https://github.com/MHSanaei/3x-ui); в Nexor добавлены подписки, REST API (`/api`) с JWT и API-ключами, фоновые задачи, опционально PostgreSQL, пути `/etc/nexor` и `/var/log/nexor`, обновлённый интерфейс.

[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)
[![GitHub](https://img.shields.io/badge/GitHub-HeliTeam%2FNexorPanel-181717?logo=github)](https://github.com/HeliTeam/NexorPanel)
[![Go module](https://img.shields.io/badge/module-github.com%2Fnexor%2Fpanel-00ADD8)](https://github.com/HeliTeam/NexorPanel)

> [!IMPORTANT]
> Используйте панель только там, где это законно. Проект рассчитан на легальные VPN/прокси под вашу ответственность.

## Возможности

- Администраторы панели и интерактивное создание первого админа: `nexor create-admin`.
- Пользователи подписок, учёт трафика и сроков действия.
- REST API с ограничением частоты запросов и защитой от перебора на чувствительных маршрутах.
- SQLite по умолчанию; PostgreSQL через переменную окружения (см. `deploy/nexor.service`).
- Примеры **systemd** и **nginx** в каталоге [`deploy/`](deploy/).

## Автоустановка на чистый сервер (Debian / Ubuntu)

Скрипт ставит зависимости, **Go** нужной версии из `go.mod`, собирает панель, настраивает **nginx** и сертификаты, затем запускает **create-admin** (ник, пароль, подтверждение).

- Режим **1** — ваш домен и **Let's Encrypt** (нужны DNS A/AAAA на сервер и открытый порт 80).
- Режим **2** — только **IP**: HTTPS на nginx с **самоподписанным** сертификатом (браузер покажет предупреждение).

Одна команда (скрипт сохраняется во временный файл, чтобы работали запросы ввода):

```bash
curl -fsSL https://raw.githubusercontent.com/HeliTeam/NexorPanel/main/deploy/install-nexor.sh -o /tmp/nexor-install.sh && sudo bash /tmp/nexor-install.sh && rm -f /tmp/nexor-install.sh
```

Локально из клона:

```bash
sudo bash deploy/install-nexor.sh
```

Панель снаружи открывается по **HTTPS** на порту **443** (прокси на локальный HTTP панели **2053**). Пути подписки **`/sub/`**, **`/json/`**, **`/clash/`** проксируются на локальный порт **2096**.

## Ручная установка (кратко)

```bash
git clone https://github.com/HeliTeam/NexorPanel.git
cd NexorPanel
sudo apt-get update
sudo apt-get install -y build-essential pkg-config libsqlite3-dev
export CGO_ENABLED=1
go build -ldflags "-w -s" -o nexor .

sudo mkdir -p /usr/local/nexor /etc/nexor /var/log/nexor
sudo cp nexor /usr/local/nexor/
sudo cp deploy/nexor.service /etc/systemd/system/nexor.service
sudo systemctl daemon-reload

export NEXOR_DB_FOLDER=/etc/nexor NEXOR_LOG_FOLDER=/var/log/nexor
sudo -E /usr/local/nexor/nexor create-admin

sudo systemctl enable --now nexor
```

Импорт в Go остаётся по пути модуля: **`github.com/nexor/panel`**.

## Переменные окружения

| Переменная | Назначение |
|------------|------------|
| `NEXOR_DB_FOLDER` / `XUI_DB_FOLDER` | Каталог с базой (часто `/etc/nexor`) |
| `NEXOR_LOG_FOLDER` / `XUI_LOG_FOLDER` | Каталог логов (например `/var/log/nexor`) |
| `NEXOR_DATABASE_URL` | Строка подключения PostgreSQL (опционально) |

## Документация по Xray и протоколам

Концепции общие с апстримом: [вики 3x-ui](https://github.com/MHSanaei/3x-ui/wiki).

Пример конфигурации nginx: [`deploy/nginx-xray.helitop.ru.conf`](deploy/nginx-xray.helitop.ru.conf).

## Особая благодарность

- **Seizure** и **[Klieer](https://github.com/klieer1337)** — спасибо за вклад и поддержку проекта. ♥
- **[HeliTeam](https://github.com/HeliTeam)** — команда, без которой разработка и публикация Nexor Panel были бы невозможны.

Отдельно — уважение к апстриму: [alireza0](https://github.com/alireza0/) и участникам [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui).

## Сторонние данные (маршрутизация)

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (GPL-3.0)
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (GPL-3.0)

---

Если проект полезен, можно поставить звезду репозиторию [HeliTeam/NexorPanel](https://github.com/HeliTeam/NexorPanel).
