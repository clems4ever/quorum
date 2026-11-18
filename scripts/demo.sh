#!/usr/bin/env sh
# Runs Quorum with the fake Google provider on a single public origin:
#   gateway (:$PORT) -> /fake-google/* fake OAuth server, everything else -> app (:$APP_PORT)
# Usage: PUBLIC_URL=https://my-tunnel.example.com scripts/demo.sh
set -eu
PORT="${PORT:-9090}"
APP_PORT="${APP_PORT:-8080}"
PUBLIC_URL="${PUBLIC_URL:-http://localhost:$PORT}"
BIN="${BIN:-./bin}"
export GOOGLE_CLIENT_ID=demo-client GOOGLE_CLIENT_SECRET=demo-secret

ADDR="127.0.0.1:$APP_PORT" \
PUBLIC_URL="$PUBLIC_URL" \
GOOGLE_AUTH_URL="$PUBLIC_URL/fake-google/o/oauth2/v2/auth" \
GOOGLE_TOKEN_URL="http://127.0.0.1:$PORT/fake-google/token" \
GOOGLE_USERINFO_URL="http://127.0.0.1:$PORT/fake-google/v1/userinfo" \
"$BIN/quorum" serve &
APP=$!
trap 'kill $APP 2>/dev/null' EXIT INT TERM

ADDR=":$PORT" \
PREFIX=/fake-google \
UPSTREAM="http://127.0.0.1:$APP_PORT" \
PUBLIC_URL="$PUBLIC_URL/fake-google" \
CLIENT_ID=demo-client CLIENT_SECRET=demo-secret \
"$BIN/fakegoogle"
