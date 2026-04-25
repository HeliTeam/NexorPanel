#!/usr/bin/env bash
# Nexor Panel — установка на Debian/Ubuntu (root).
# Стиль: этапы с пояснениями, прогресс, без лишнего шума где возможно.

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

# ── оформление (только если TTY) ─────────────────────────────────
if [[ -t 1 ]]; then
  B='\033[1m'
  D='\033[2m'
  G='\033[32m'
  C='\033[36m'
  Y='\033[33m'
  R='\033[31m'
  Z='\033[0m'
else
  B=D=G=C=Y=R=Z=''
fi

die() { echo -e "${R}Ошибка:${Z} $*" >&2; exit 1; }

tip() { echo -e "  ${D}→ $*${Z}"; }

hdr() {
  echo ""
  echo -e "${C}${B}$*${Z}"
}

progress_bar() {
  local cur=$1 total=$2 width=${3:-36}
  local n=$((cur * width / total))
  local i
  printf '  ['
  for ((i = 0; i < width; i++)); do
    if ((i < n)); then printf '%s' '█'; else printf '%s' '░'; fi
  done
  printf '] %s/%s\n' "$cur" "$total"
}

phase() {
  STEP=$((STEP + 1))
  echo ""
  echo -e "${B}[этап $STEP/$STEPS]${Z} $1"
  progress_bar "$STEP" "$STEPS"
  tip "$2"
}

# Фоновая команда + спиннер (ASCII)
run_spinner() {
  local msg="$1"
  shift
  "$@" &
  local pid=$!
  local spin='|/-\'
  local i=0
  while kill -0 "$pid" 2>/dev/null; do
    printf '\r  %s%c%s %s' "$Y" "${spin:i:1}" "$Z" "$msg"
    i=$(( (i + 1) % 4 ))
    sleep 0.12
  done
  printf '\r\033[K'
  wait "$pid"
}

[[ "$(id -u)" -eq 0 ]] || die "Запустите от root: sudo bash $0"

echo -e "${B}"
echo "  ═══════════════════════════════════════════════════════"
echo "           NEXOR VPN PANEL — мастер установки"
echo "  ═══════════════════════════════════════════════════════"
echo -e "${Z}"
tip "Панель ставится на этот сервер: веб-интерфейс, Xray, подписки. Всё в одном скрипте."
tip "Если что-то пойдёт не так — пришлите вывод и файл /tmp/nexor-install.log (он создаётся на этапах apt/сборки)."

INSTALL_LOG="/tmp/nexor-install.log"
: >"$INSTALL_LOG"

echo ""
echo -e "${B}Доступ к панели с интернета${Z}"
echo "  ${G}1${Z} — есть домен → HTTPS через Let's Encrypt (нужен DNS на этот сервер, порт 80)."
echo "  ${G}2${Z} — только IP → HTTPS с самоподписанным сертификатом (браузер предупредит — это нормально)."
read -r -p "Выберите 1 или 2: " MODE
[[ "$MODE" == "1" || "$MODE" == "2" ]] || die "Введите 1 или 2"

CERT_EMAIL=""
if [[ "$MODE" == "1" ]]; then
  read -r -p "Домен (например panel.example.com): " DOMAIN
  DOMAIN="${DOMAIN// /}"
  [[ -n "$DOMAIN" ]] || die "Домен не может быть пустым"
  read -r -p "Email для Let's Encrypt: " CERT_EMAIL
  [[ -n "$CERT_EMAIL" ]] || die "Email нужен для certbot"
else
  read -r -p "Публичный IP (Enter — авто): " SERVER_IP
  SERVER_IP="${SERVER_IP// /}"
  if [[ -z "$SERVER_IP" ]]; then
    SERVER_IP="$(curl -fsS --connect-timeout 5 https://api.ipify.org 2>/dev/null || true)"
  fi
  [[ -n "$SERVER_IP" ]] || die "Укажите IP вручную и запустите скрипт снова"
  DOMAIN=""
fi

# С certbot на один этап больше, чем режим «только IP»
if [[ "$MODE" == "1" ]]; then STEPS=9; else STEPS=8; fi
STEP=0

export DEBIAN_FRONTEND=noninteractive
# Меньше вопросов от needrestart на Ubuntu
export NEEDRESTART_MODE=a 2>/dev/null || true

phase "Системные пакеты" \
  "Ставим компилятор, SQLite для Go, nginx — это одноразовая загрузка. Дальше быстрее."

APT_PKGS="git curl ca-certificates openssl build-essential pkg-config libsqlite3-dev nginx"
[[ "$MODE" == "1" ]] && APT_PKGS="$APT_PKGS certbot python3-certbot-nginx"

{
  apt-get update -qq
  # --no-install-recommends ускоряет и уменьшает объём
  apt-get install -y -qq --no-install-recommends $APT_PKGS
} >>"$INSTALL_LOG" 2>&1 || {
  echo -e "${R}apt завершился с ошибкой. Хвост лога:${Z}"
  tail -30 "$INSTALL_LOG"
  die "Смотрите полный лог: $INSTALL_LOG"
}
echo -e "  ${G}Готово.${Z} Пакеты установлены."

phase "Исходный код Nexor" \
  "Клонируем или обновляем репозиторий с GitHub (только последний коммит ветки — быстро)."

export GIT_TERMINAL_PROMPT=0
if [[ -d "$INSTALL_SRC/.git" ]]; then
  run_spinner "git fetch / reset…" bash -c "git -C '$INSTALL_SRC' fetch --depth 1 origin '$REPO_BRANCH' && git -C '$INSTALL_SRC' reset --hard 'origin/$REPO_BRANCH'"
else
  run_spinner "git clone…" git clone --depth 1 --single-branch --branch "$REPO_BRANCH" "$REPO_URL" "$INSTALL_SRC"
fi
echo -e "  ${G}Готово.${Z} Код в $INSTALL_SRC"

GO_MOD_VER="$(grep -E '^go[[:space:]]+[0-9]' "$INSTALL_SRC/go.mod" | awk '{print $2}')"
[[ -n "$GO_MOD_VER" ]] || die "Не прочитана версия Go из go.mod"

case "$(uname -m)" in
  x86_64) GOARCH_DL=amd64 ;;
  aarch64) GOARCH_DL=arm64 ;;
  armv7l) GOARCH_DL=armv6l ;;
  *) die "Архитектура не поддерживается: $(uname -m)" ;;
esac

install_go_tarball() {
  local ver="$1" tmp
  tmp="$(mktemp -d)"
  # --progress-bar даёт один ползунок вместо тысячи строк
  curl -fL --progress-bar "https://go.dev/dl/go${ver}.linux-${GOARCH_DL}.tar.gz" -o "$tmp/go.tgz"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "$tmp/go.tgz"
  rm -rf "$tmp"
}

need_go_download() {
  if ! command -v go >/dev/null 2>&1; then return 0; fi
  local have min
  have="$(go version | sed -E 's/.*go([0-9.]+).*/\1/')"
  min="$(printf '%s\n' "$have" "$GO_MOD_VER" | sort -V | head -1)"
  [[ "$min" != "$GO_MOD_VER" ]]
}

phase "Компилятор Go $GO_MOD_VER" \
  "Go нужен один раз: им собирается бинарник панели. Если версия уже подходит — пропускаем скачивание."

if need_go_download; then
  echo "  Скачивание Go $GO_MOD_VER …"
  install_go_tarball "$GO_MOD_VER"
else
  tip "Подходящий Go уже в PATH — tarball не качаем."
fi
export PATH="/usr/local/go/bin:$PATH"
command -v go >/dev/null || die "Go не найден"
go version | sed 's/^/  /'

phase "Сборка бинарника nexor" \
  "go mod tidy подтягивает checksums; go build компилирует (обычно 1–3 минуты). Можно заварить чай."

cd "$INSTALL_SRC"
export CGO_ENABLED=1
export GOWORK=off
# Кэш модулей на диске ускорит повторные установки
export GOMODCACHE="${GOMODCACHE:-$HOME/go/pkg/mod}"

echo "--- go build $(date -Iseconds) ---" >>"$INSTALL_LOG"
run_spinner "go mod tidy…" bash -c "go mod tidy >>'$INSTALL_LOG' 2>&1" || {
  echo -e "${R}go mod tidy завершился с ошибкой. Хвост лога:${Z}"
  tail -40 "$INSTALL_LOG"
  die "Полный лог: $INSTALL_LOG"
}
run_spinner "go build (1–3 мин)…" bash -c "go build -ldflags '-w -s' -o nexor . >>'$INSTALL_LOG' 2>&1" || {
  echo -e "${R}Сборка упала. Фрагмент лога:${Z}"
  tail -40 "$INSTALL_LOG"
  die "Полный лог: $INSTALL_LOG"
}
echo -e "  ${G}Готово.${Z} Сборка завершена."

phase "Каталоги и systemd" \
  "Бинарник в /usr/local/nexor, база в $DB_DIR, логи в $LOG_DIR. Сервис перезапустится в конце."

install -d -m 0755 "$BIN_DIR" "$DB_DIR" "$LOG_DIR"
install -m 0755 nexor "$BIN_DIR/nexor"

sed -e "s|^User=.*|User=$NEXOR_USER|" \
    -e "s|^Environment=NEXOR_DB_FOLDER=.*|Environment=NEXOR_DB_FOLDER=$DB_DIR|" \
    -e "s|^Environment=NEXOR_LOG_FOLDER=.*|Environment=NEXOR_LOG_FOLDER=$LOG_DIR|" \
  "$INSTALL_SRC/deploy/nexor.service" >/etc/systemd/system/nexor.service

systemctl daemon-reload
systemctl stop nexor 2>/dev/null || true
echo -e "  ${G}Готово.${Z} unit-файл установлен."

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

phase "Nginx как обратный прокси" \
  "Снаружи только 80/443. Панель слушает localhost:2053, подписка :2096 — nginx передаёт трафик внутрь."

if [[ "$MODE" == "1" ]]; then
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
echo -e "  ${G}Готово.${Z} Nginx перезагружен."

if [[ "$MODE" == "1" ]]; then
  phase "Сертификат Let's Encrypt" \
    "Certbot проверит домен через порт 80 и включит HTTPS. Убедитесь, что DNS уже указывает на этот сервер."

  certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos -m "$CERT_EMAIL" --redirect >>"$INSTALL_LOG" 2>&1 || {
    tail -25 "$INSTALL_LOG"
    die "Certbot не выдал сертификат (DNS, файрвол 80, см. $INSTALL_LOG)"
  }
  echo -e "  ${G}Готово.${Z} TLS активен."
fi

echo ""
echo -e "${B}[дополнительно]${Z} файрвол ufw"
read -r -p "Открыть порты 80 и 443 в ufw? [y/N]: " OPEN_UFW
if [[ "${OPEN_UFW:-}" =~ ^[yY]$ ]] && command -v ufw >/dev/null 2>&1; then
  ufw allow 80/tcp >/dev/null 2>&1 || true
  ufw allow 443/tcp >/dev/null 2>&1 || true
  echo -e "  ${G}Готово.${Z} Правила добавлены (при необходимости: ufw enable)."
fi

export NEXOR_DB_FOLDER="$DB_DIR"
export NEXOR_LOG_FOLDER="$LOG_DIR"

phase "Первый администратор" \
  "Сейчас откроется create-admin: поле «Nickname» — это ваш логин в панель, затем пароль дважды."

"$BIN_DIR/nexor" create-admin || die "create-admin прерван"

phase "Запуск сервиса nexor" \
  "Панель и подписка поднимаются как systemd-сервис и стартуют после перезагрузки сервера."

systemctl enable nexor
systemctl start nexor
sleep 2
systemctl is-active --quiet nexor || die "nexor не активен — journalctl -u nexor -e"

echo ""
echo -e "${G}${B}"
echo "  ═══════════════════════════════════════════════════════"
echo "              УСТАНОВКА ЗАВЕРШЕНА"
echo "  ═══════════════════════════════════════════════════════"
echo -e "${Z}"
if [[ "$MODE" == "1" ]]; then
  echo -e "  ${B}Откройте:${Z}  ${C}https://${DOMAIN}/${Z}"
else
  echo -e "  ${B}Откройте:${Z}  ${C}https://${SERVER_IP}/${Z}"
  tip "Самоподписанный сертификат — нажмите «Дополнительно» → перейти в браузере."
fi
tip "Логин = ник из create-admin. Команда статуса: systemctl status nexor"
tip "Лог установки: $INSTALL_LOG"
echo ""
