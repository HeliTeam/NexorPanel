#!/usr/bin/env bash
# Nexor Panel — установка на чистый Debian/Ubuntu (root).
# Домен + Let's Encrypt или IP + самоподписанный сертификат на nginx.
# Панель: HTTP 127.0.0.1:2053, подписка: 127.0.0.1:2096; TLS на nginx.

set -euo pipefail

REPO_URL="${NEXOR_REPO_URL:-https://github.com/HeliTeam/NexorPanel.git}"
REPO_BRANCH="${NEXOR_REPO_BRANCH:-main}"
INSTALL_SRC="/usr/local/src/NexorPanel"
BIN_DIR="/usr/local/nexor"
DB_DIR="/etc/nexor"
LOG_DIR="/var/log/nexor"
PANEL_PORT="2053"
SUB_PORT="2096"
NEXOR_USER="${NEXOR_USER:-root}"

die() { echo "Ошибка: $*" >&2; exit 1; }

[[ "$(id -u)" -eq 0 ]] || die "Запустите от root: sudo bash $0"

echo "=========================================="
echo "  Nexor VPN Panel — установка"
echo "=========================================="
echo ""
echo "Как будем заходить в панель снаружи?"
echo "  1 — есть домен (HTTPS через Let's Encrypt)"
echo "  2 — только IP (HTTPS nginx с самоподписанным сертификатом)"
read -r -p "Выберите 1 или 2 [1/2]: " MODE
[[ "$MODE" == "1" || "$MODE" == "2" ]] || die "Нужно ввести 1 или 2"

CERT_EMAIL=""
if [[ "$MODE" == "1" ]]; then
  read -r -p "Домен панели (например panel.example.com): " DOMAIN
  DOMAIN="${DOMAIN// /}"
  [[ -n "$DOMAIN" ]] || die "Домен не может быть пустым"
  read -r -p "Email для Let's Encrypt: " CERT_EMAIL
  [[ -n "$CERT_EMAIL" ]] || die "Email обязателен для certbot"
else
  read -r -p "Публичный IP сервера (Enter — определить автоматически): " SERVER_IP
  SERVER_IP="${SERVER_IP// /}"
  if [[ -z "$SERVER_IP" ]]; then
    SERVER_IP="$(curl -fsS --connect-timeout 5 https://api.ipify.org 2>/dev/null || true)"
  fi
  [[ -n "$SERVER_IP" ]] || die "Не удалось определить IP — введите вручную и запустите снова"
  DOMAIN=""
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq \
  git curl ca-certificates gnupg openssl \
  build-essential pkg-config libsqlite3-dev \
  nginx

if [[ -d "$INSTALL_SRC/.git" ]]; then
  git -C "$INSTALL_SRC" fetch --depth 1 origin "$REPO_BRANCH"
  git -C "$INSTALL_SRC" reset --hard "origin/$REPO_BRANCH"
else
  rm -rf "$INSTALL_SRC"
  git clone --depth 1 --branch "$REPO_BRANCH" "$REPO_URL" "$INSTALL_SRC"
fi

GO_MOD_VER="$(grep -E '^go[[:space:]]+[0-9]' "$INSTALL_SRC/go.mod" | awk '{print $2}')"
[[ -n "$GO_MOD_VER" ]] || die "Не удалось прочитать версию Go из go.mod"
echo "Требуется Go $GO_MOD_VER (из go.mod)"

GOARCH_DL=""
case "$(uname -m)" in
  x86_64) GOARCH_DL=amd64 ;;
  aarch64) GOARCH_DL=arm64 ;;
  armv7l) GOARCH_DL=armv6l ;;
  *) die "Неподдерживаемая архитектура: $(uname -m)" ;;
esac

install_go_tarball() {
  local ver="$1"
  local tmp
  tmp="$(mktemp -d)"
  curl -fsSL "https://go.dev/dl/go${ver}.linux-${GOARCH_DL}.tar.gz" -o "$tmp/go.tgz"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "$tmp/go.tgz"
  rm -rf "$tmp"
}

need_go_download() {
  if ! command -v go >/dev/null 2>&1; then
    return 0
  fi
  local have min
  have="$(go version | sed -E 's/.*go([0-9.]+).*/\1/')"
  min="$(printf '%s\n' "$have" "$GO_MOD_VER" | sort -V | head -1)"
  [[ "$min" != "$GO_MOD_VER" ]]
}

if need_go_download; then
  echo "Устанавливаю Go $GO_MOD_VER в /usr/local/go ..."
  install_go_tarball "$GO_MOD_VER"
fi
export PATH="/usr/local/go/bin:$PATH"
command -v go >/dev/null || die "Go не найден"
go version

cd "$INSTALL_SRC"
export CGO_ENABLED=1
export GOWORK=off
# Важно: go mod download ДО tidy часто падает с «updates to go.mod needed» и при set -e скрипт
# не доходит до tidy. Сначала приводим go.sum в порядок, потом собираем.
go mod tidy
go build -ldflags "-w -s" -o nexor .

install -d -m 0755 "$BIN_DIR" "$DB_DIR" "$LOG_DIR"
install -m 0755 nexor "$BIN_DIR/nexor"

sed -e "s|^User=.*|User=$NEXOR_USER|" \
    -e "s|^Environment=NEXOR_DB_FOLDER=.*|Environment=NEXOR_DB_FOLDER=$DB_DIR|" \
    -e "s|^Environment=NEXOR_LOG_FOLDER=.*|Environment=NEXOR_LOG_FOLDER=$LOG_DIR|" \
  "$INSTALL_SRC/deploy/nexor.service" > /etc/systemd/system/nexor.service

systemctl daemon-reload
systemctl stop nexor 2>/dev/null || true

PROXY_BLOCK="$(cat <<'PX'
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 120s;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
PX
)"

NGINX_SITE="/etc/nginx/sites-available/nexor.conf"
NGINX_EN="/etc/nginx/sites-enabled/nexor.conf"

if [[ "$MODE" == "1" ]]; then
  apt-get install -y -qq certbot python3-certbot-nginx
  cat >"$NGINX_SITE" <<EOF
upstream nexor_panel {
    server 127.0.0.1:${PANEL_PORT};
}
upstream nexor_sub {
    server 127.0.0.1:${SUB_PORT};
}

server {
    listen 80;
    listen [::]:80;
    server_name ${DOMAIN};
    client_max_body_size 32m;

    location / {
        proxy_pass http://nexor_panel;
${PROXY_BLOCK}
    }
    location /sub/ {
        proxy_pass http://nexor_sub;
${PROXY_BLOCK}
    }
    location /json/ {
        proxy_pass http://nexor_sub;
${PROXY_BLOCK}
    }
    location /clash/ {
        proxy_pass http://nexor_sub;
${PROXY_BLOCK}
    }
}
EOF
else
  install -d -m 0755 /etc/ssl/nexor
  openssl req -x509 -nodes -newkey rsa:2048 -days 825 \
    -keyout /etc/ssl/nexor/self.key \
    -out /etc/ssl/nexor/self.crt \
    -subj "/CN=${SERVER_IP}" 2>/dev/null

  cat >"$NGINX_SITE" <<EOF
upstream nexor_panel {
    server 127.0.0.1:${PANEL_PORT};
}
upstream nexor_sub {
    server 127.0.0.1:${SUB_PORT};
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name ${SERVER_IP};
    ssl_certificate     /etc/ssl/nexor/self.crt;
    ssl_certificate_key /etc/ssl/nexor/self.key;
    client_max_body_size 32m;

    location / {
        proxy_pass http://nexor_panel;
${PROXY_BLOCK}
    }
    location /sub/ {
        proxy_pass http://nexor_sub;
${PROXY_BLOCK}
    }
    location /json/ {
        proxy_pass http://nexor_sub;
${PROXY_BLOCK}
    }
    location /clash/ {
        proxy_pass http://nexor_sub;
${PROXY_BLOCK}
    }
}

server {
    listen 80;
    listen [::]:80;
    server_name ${SERVER_IP};
    return 301 https://\$host\$request_uri;
}
EOF
fi

rm -f /etc/nginx/sites-enabled/default
ln -sf "$NGINX_SITE" "$NGINX_EN"
nginx -t
systemctl reload nginx

if [[ "$MODE" == "1" ]]; then
  certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos -m "$CERT_EMAIL" --redirect || \
    die "Certbot не выдал сертификат (DNS A/AAAA на этот сервер, порт 80 снаружи)"
fi

read -r -p "Открыть 80/tcp и 443/tcp в ufw? [y/N]: " OPEN_UFW
if [[ "${OPEN_UFW:-}" =~ ^[yY]$ ]] && command -v ufw >/dev/null 2>&1; then
  ufw allow 80/tcp || true
  ufw allow 443/tcp || true
  echo "Правила ufw добавлены."
fi

export NEXOR_DB_FOLDER="$DB_DIR"
export NEXOR_LOG_FOLDER="$LOG_DIR"

echo ""
echo "=========================================="
echo "  Первый администратор панели"
echo "=========================================="
echo "Сейчас запустится create-admin: введите ник (логин), пароль и подтверждение."
echo ""
"$BIN_DIR/nexor" create-admin || die "create-admin не выполнен"

systemctl enable nexor
systemctl start nexor
sleep 2
systemctl is-active --quiet nexor || die "nexor не запустился — journalctl -u nexor"

echo ""
echo "=========================================="
echo "  Готово"
echo "=========================================="
if [[ "$MODE" == "1" ]]; then
  echo "Откройте в браузере: https://${DOMAIN}/"
else
  echo "Откройте в браузере: https://${SERVER_IP}/"
  echo "(Предупреждение о сертификате — нормально для самоподписанного.)"
fi
echo "Логин — тот ник, что вы указали (в программе поле «Nickname»)."
echo "Статус: systemctl status nexor"
echo ""
