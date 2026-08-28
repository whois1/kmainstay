#!/usr/bin/env bash
set -euo pipefail
repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
run_dir=$(mktemp -d)
port=${KMAINSTAY_TEST_PORT:-18765}
base_url="http://127.0.0.1:${port}"
server_pid=""; bot_pid=""
cleanup() { [[ -z "$bot_pid" ]] || kill "$bot_pid" 2>/dev/null || true; [[ -z "$server_pid" ]] || kill "$server_pid" 2>/dev/null || true; rm -rf "$run_dir"; }
trap cleanup EXIT
cd "$repo_dir"
npm run build
go build -o "$run_dir/kmainstay" ./cmd/kmainstay
go build -o "$run_dir/kmainstay-initialise" ./cmd/kmainstay-initialise
printf '%s\n' 'correct horse battery staple' | DB_PATH="$run_dir/test.db" BOOTSTRAP_EMAIL=michael@example.com BOOTSTRAP_NAME=Michael BOOTSTRAP_ORGANISATION=Mainstay "$run_dir/kmainstay-initialise" >/dev/null
DB_PATH="$run_dir/test.db" LISTEN_ADDR="127.0.0.1:${port}" INSECURE_COOKIES=1 "$run_dir/kmainstay" >"$run_dir/server.log" 2>&1 & server_pid=$!
for _ in {1..50}; do curl -fsS "$base_url/healthz" >/dev/null 2>&1 && break; sleep .1; done
curl -fsS -c "$run_dir/cookies" -H "Origin: $base_url" -H 'Content-Type: application/json' -d '{"email":"michael@example.com","password":"correct horse battery staple"}' "$base_url/api/session" >/dev/null
organisations=$(curl -fsS -b "$run_dir/cookies" "$base_url/api/organisations")
organisation_id=$(node -e 'const v=JSON.parse(process.argv[1]);process.stdout.write(v[0].id)' "$organisations")
conversations=$(curl -fsS -b "$run_dir/cookies" "$base_url/api/organisations/$organisation_id/conversations")
conversation_id=$(node -e 'const v=JSON.parse(process.argv[1]);process.stdout.write(v[0].id)' "$conversations")
bot_created=$(curl -fsS -b "$run_dir/cookies" -H "Origin: $base_url" -H 'Content-Type: application/json' -d '{"name":"Hector"}' "$base_url/api/organisations/$organisation_id/bots")
api_key=$(node -e 'const v=JSON.parse(process.argv[1]);process.stdout.write(v.api_key)' "$bot_created")
KMAINSTAY_URL="$base_url" KMAINSTAY_API_KEY="$api_key" node examples/reference-bot.mjs >"$run_dir/bot.log" 2>&1 & bot_pid=$!
sleep .3
curl -fsS -b "$run_dir/cookies" -H "Origin: $base_url" -H 'Content-Type: application/json' -d '{"body":"Hello @Hector","client_id":"local-exchange-human"}' "$base_url/api/conversations/$conversation_id/messages" >/dev/null
for _ in {1..50}; do
  history=$(curl -fsS -b "$run_dir/cookies" "$base_url/api/conversations/$conversation_id/messages")
  if node -e 'const v=JSON.parse(process.argv[1]);process.exit(v.some(m=>m.author_kind==="bot"&&m.body==="Hector received: Hello @Hector")?0:1)' "$history"; then echo "reference exchange passed: Michael -> Hector -> Michael"; exit 0; fi
  sleep .1
done
echo "reference exchange failed" >&2; sed -n '1,120p' "$run_dir/server.log" >&2; sed -n '1,120p' "$run_dir/bot.log" >&2; exit 1
