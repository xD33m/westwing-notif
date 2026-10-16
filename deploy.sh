#!/usr/bin/env bash
# Build the arm64 binary locally, copy it to the host, (re)start the container.
#
# Usage: bash deploy.sh
# Target is read from deploy.env (see deploy.env.example) or the HOST / DIR
# environment variables. Never overwrites .env or config/items.txt on the host.
set -euo pipefail
cd "$(dirname "$0")"

if [ -f deploy.env ]; then
  # shellcheck disable=SC1091
  . ./deploy.env
fi
HOST=${HOST:-pi}
DIR=${DIR:-westwing-notif}

go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o build/westwing-notif .

ssh "$HOST" "mkdir -p $DIR/config $DIR/data"
scp -q build/westwing-notif Dockerfile .dockerignore docker-compose.yml "$HOST:$DIR/"
# items.txt is only seeded: once on the host, it's edited there.
ssh "$HOST" "test -f $DIR/config/items.txt || cat > $DIR/config/items.txt" < config/items.txt

ssh "$HOST" "DIR=$DIR bash -s" <<'REMOTE'
set -euo pipefail
cd "$HOME/$DIR"
chmod +x westwing-notif
printf 'westwing-notif\ndata/\n.env\n' > .gitignore
if [ ! -f .env ]; then
  topic="westwing-$(od -An -tx1 -N8 /dev/urandom | tr -d " \n")"
  echo "NTFY_URL=https://ntfy.sh/$topic" > .env
  echo "Created a new random ntfy topic."
fi
docker compose up -d --build
echo
echo "Subscribe in the ntfy app to: $(sed -n 's/^NTFY_URL=//p' .env)"
REMOTE
