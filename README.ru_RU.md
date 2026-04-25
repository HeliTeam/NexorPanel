[English](README.md) | [Русский](README.ru_RU.md)

<p align="center">
  <img alt="Nexor" src="./media/logo_nexor.png" width="160" height="160">
</p>

# Nexor VPN Panel

**Nexor** — веб-панель управления **Xray-core**, форк [3x-ui](https://github.com/MHSanaei/3x-ui). Репозиторий **[HeliTeam/NexorPanel](https://github.com/HeliTeam/NexorPanel)** добавляет собственный слой: пользователи подписок, REST API (`/api`) с JWT и API-ключами, фоновые задачи, опционально PostgreSQL, брендированные пути (`/etc/nexor`, `/var/log/nexor`) и обновлённый интерфейс.

[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)
[![Репозиторий](https://img.shields.io/badge/GitHub-HeliTeam%2FNexorPanel-181717?logo=github)](https://github.com/HeliTeam/NexorPanel)
[![Go module](https://img.shields.io/badge/module-github.com%2Fnexor%2Fpanel-00ADD8)](https://github.com/HeliTeam/NexorPanel)

> [!IMPORTANT]
> Используйте только там, где это законно. Панель рассчитана на легальные и разрешённые вам VPN/прокси-операции.

## Возможности

- **Администраторы** — операторы панели; первый админ создаётся командой `create-admin`.
- **Пользователи подписок** — модель Nexor, связанная с клиентами Xray, учёт трафика и сроков.
- **REST API** — JWT и обновление токена, API keys, ограничение частоты запросов и защита от перебора на чувствительных маршрутах.
- **Автоматизация** — периодические задачи (трафик, истечение подписки, неактивность и др.).
- **База данных** — по умолчанию SQLite; PostgreSQL через DSN (см. комментарий в [`deploy/nexor.service`](deploy/nexor.service)).
- **Развёртывание** — пример unit-файла **systemd** и фрагменты nginx в каталоге [`deploy/`](deploy/).

**Языки документации:** только **английский** ([README.md](README.md)) и **русский** (этот файл).

## Требования

- **Go** — версия в духе указанной в [`go.mod`](go.mod) (toolchain может подтянуть новый патч).
- **CGO** — для сборки с SQLite по умолчанию; на Linux нужны компилятор C и заголовки SQLite (`libsqlite3-dev` или аналог).
- **Права** — в примере сервис запускается от `root`; при необходимости замените пользователя и права на каталоги.

## Быстрый старт (сборка из исходников, Linux)

```bash
git clone https://github.com/HeliTeam/NexorPanel.git
cd NexorPanel

# Зависимости (пример Debian/Ubuntu)
sudo apt-get update
sudo apt-get install -y build-essential pkg-config libsqlite3-dev

export CGO_ENABLED=1
go build -ldflags "-w -s" -o nexor .

sudo mkdir -p /usr/local/nexor /etc/nexor /var/log/nexor
sudo cp nexor /usr/local/nexor/
sudo cp deploy/nexor.service /etc/systemd/system/nexor.service
sudo systemctl daemon-reload

# Первый администратор (интерактивно) — до или после включения сервиса
sudo /usr/local/nexor/nexor create-admin

sudo systemctl enable --now nexor
sudo systemctl status nexor --no-pager
```

Откройте панель в браузере: `http://ВАШ_СЕРВЕР:ПОРТ/` (порт задаётся в настройках панели). Откройте этот порт (и порт подписки, если используете отдельный sub-сервер) в файрволе и у облачного провайдера.

Переменные окружения (см. [`deploy/nexor.service`](deploy/nexor.service)):

- `NEXOR_DB_FOLDER` / `XUI_DB_FOLDER` — каталог базы (в примере `/etc/nexor`).
- `NEXOR_LOG_FOLDER` / `XUI_LOG_FOLDER` — каталог логов (например `/var/log/nexor`).
- По желанию: `NEXOR_DATABASE_URL` для PostgreSQL.

Готовые **one-click** установщики апстрима рассчитаны на раскладку `x-ui`; для продакшена Nexor используйте собранный бинарник и `nexor.service`.

**Путь модуля Go:** `github.com/nexor/panel`.

## Дополнительно

- Концепции протоколов и Xray (апстрим): [вики 3x-ui](https://github.com/MHSanaei/3x-ui/wiki)
- Пример обратного прокси: [`deploy/nginx-xray.helitop.ru.conf`](deploy/nginx-xray.helitop.ru.conf)

## Особая благодарность

- **Seizure** и **Klieer** — огромное спасибо за вклад и поддержку проекта. ♥
- **[HeliTeam](https://github.com/HeliTeam)** — команда, без которой разработка и выкладка этой панели были бы куда сложнее.

Отдельно — признательность апстриму: [alireza0](https://github.com/alireza0/) и участникам [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui).

## Сторонние данные (маршрутизация)

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (лицензия **GPL-3.0**)
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (лицензия **GPL-3.0**)

## Звёзды со временем

[![Stargazers over time](https://starchart.cc/HeliTeam/NexorPanel.svg?variant=adaptive)](https://starchart.cc/HeliTeam/NexorPanel)

Если Nexor вам полезен, можно поставить звезду репозиторию на GitHub.
