#!/usr/bin/env bash
# regenerates demo/demo.gif — needs vhs, ttyd, ffmpeg and a built portside
set -euo pipefail
cd "$(dirname "$0")/.."

D=/tmp/portside-demo
rm -rf "$D"
mkdir -p "$D/home/.ssh" "$D/home/Downloads" "$D/srv/app/src" "$D/srv/app/config" "$D/srv/app/logs" "$D/srv/backups"
printf 'Host prod-server\n    HostName 127.0.0.1\n    Port 2222\n' > "$D/home/.ssh/config"
(
    cd "$D/srv"
    echo 'package main' | tee app/src/main.go app/src/api.go app/src/handlers.go >/dev/null
    echo 'env: production' > app/config/production.yaml
    for i in $(seq 1 200); do echo "2026-06-10 12:00:00 INFO request served"; done > app/logs/app.log
    echo '# app' > app/README.md
    echo 'services:' > app/docker-compose.yml
    head -c 2M /dev/urandom > backups/db-2026-06-09.tar.gz
    echo 'server {}' > nginx.conf
)

go build -o /tmp/portside-demo/portside .
go build -o /tmp/portside-demo/sshserver ./demo/sshserver

(cd "$D/srv" && "$D/sshserver") &
SRV=$!
trap 'kill $SRV 2>/dev/null' EXIT
sleep 1

PATH="$D:$PATH" vhs demo/demo.tape
