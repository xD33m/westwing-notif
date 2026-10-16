#!/usr/bin/env bash
# Build the arm64 binary here, ship it to the Pi, (re)start the container.
# Usage: bash deploy.sh            (HOST and DIR overridable via env)
set -euo pipefail

HOST=${HOST:-pi}
DIR=${DIR:-westwing-notif}
cd "$(dirname "$0")"

go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o build/westwing-notif .

ssh "$HOST" "mkdir -p $DIR/config $DIR/data"
scp -q build/westwing-notif Dockerfile docker-compose.yml "$HOST:$DIR/"
# items.txt is only seeded: once on the Pi, it's edited there.
ssh "$HOST" "test -f $DIR/config/items.txt || cat > $DIR/config/items.txt" < config/items.txt

ssh "$HOST" "DIR=$DIR bash -s" <<'REMOTE'
set -euo pipefail
cd "$HOME/$DIR"
chmod +x westwing-notif
printf 'westwing-notif\ndata/\n.env\n' > .gitignore
if [ ! -f .env ]; then
  topic="westwing-$(od -An -tx1 -N8 /dev/urandom | tr -d " \n")"
  echo "NTFY_URL=https://ntfy.sh/$topic" > .env
  echo "Neues ntfy-Topic angelegt: $topic"
fi
docker compose up -d --build
echo
grep NTFY_URL .env
echo "In der ntfy-App abonnieren: Server ntfy.sh, Topic ${topic:-$(sed 's|.*/||' .env)}"
REMOTE
