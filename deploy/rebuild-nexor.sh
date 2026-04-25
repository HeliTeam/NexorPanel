#!/usr/bin/env bash
# Пересборка бинарника панели после git pull.
# На 1 vCPU / 1 ГБ ОЗУ сборка часто 10–30+ мин — это норма; ход виден в логе, не нажимайте Ctrl+C.
set -euo pipefail
set -o pipefail
export PATH="/usr/local/go/bin:$PATH"
export CGO_ENABLED=1
export GOWORK=off
SRC="${NEXOR_SRC:-/usr/local/src/NexorPanel}"
cd "$SRC" || { echo "Нет каталога $SRC — задайте NEXOR_SRC=..." >&2; exit 1; }

NCPU=$(nproc 2>/dev/null || echo 1)
if (( NCPU <= 2 )); then
  export GOMAXPROCS="${GOMAXPROCS:-1}"
  POPT=(-p 1)
else
  POPT=()
fi
if [[ -r /proc/meminfo ]]; then
  M=$(awk '/^MemTotal:/{print $2; exit}' /proc/meminfo)
  if (( M < 1600000 )); then
    export GOGC="${GOGC:-20}"
  elif (( M < 2560000 )); then
    export GOGC="${GOGC:-30}"
  fi
fi

LOG="${NEXOR_REBUILD_LOG:-/tmp/nexor-rebuild.log}"
echo "→ $SRC — go build (10–30+ мин на 1 vCPU, не прерывайте). Вывод: экран + $LOG"
go build -v -trimpath -buildvcs=false -ldflags "-w -s" "${POPT[@]}" -o /usr/local/nexor/nexor . 2>&1 | tee "$LOG"

systemctl restart nexor
echo "Готово. Проверка: systemctl status nexor --no-pager"
