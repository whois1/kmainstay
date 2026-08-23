#!/usr/bin/env bash
set -Eeuo pipefail

readonly application_upload=${1:-}
readonly initialisation_upload=${2:-}
readonly expected_application_sha256=${3:-}
readonly expected_initialisation_sha256=${4:-}
readonly application_path=/opt/kmainstay/kmainstay
readonly initialisation_path=/opt/kmainstay/kmainstay-initialise
readonly data_directory=/var/lib/kmainstay
readonly attachment_directory="$data_directory/uploads"
readonly backup_root=/var/lib/kmainstay/deployment-backups
readonly health_url=http://127.0.0.1:8080/healthz
readonly deploy_home=/home/kmainstay-deploy

if [[ $# -ne 4 ]]; then
  printf 'usage: install-release.sh APPLICATION INITIALISATION APPLICATION_SHA256 INITIALISATION_SHA256\n' >&2
  exit 2
fi

for upload in "$application_upload" "$initialisation_upload"; do
  resolved_upload=$(readlink -f -- "$upload")
  if [[ "$resolved_upload" != "$deploy_home/"* || ! -f "$resolved_upload" || -L "$upload" ]]; then
    printf 'refusing unexpected upload path: %s\n' "$upload" >&2
    exit 2
  fi
done

for checksum in "$expected_application_sha256" "$expected_initialisation_sha256"; do
  if [[ ! "$checksum" =~ ^[0-9a-f]{64}$ ]]; then
    printf 'invalid SHA-256 checksum\n' >&2
    exit 2
  fi
done

actual_application_sha256=$(sha256sum -- "$application_upload" | cut -d' ' -f1)
actual_initialisation_sha256=$(sha256sum -- "$initialisation_upload" | cut -d' ' -f1)
if [[ "$actual_application_sha256" != "$expected_application_sha256" || "$actual_initialisation_sha256" != "$expected_initialisation_sha256" ]]; then
  printf 'uploaded release checksum mismatch\n' >&2
  exit 1
fi

exec 9>/run/lock/kmainstay-deploy.lock
flock -n 9 || { printf 'another deployment is running\n' >&2; exit 1; }

readonly deployed_at=$(date -u +%Y%m%dT%H%M%SZ)
readonly backup_directory="$backup_root/$deployed_at"
mkdir -p "$backup_directory"
install -d -o kmainstay -g kmainstay -m 0700 "$attachment_directory"

systemctl stop kmainstay
trap 'systemctl start kmainstay >/dev/null 2>&1 || true' EXIT
cp --preserve=mode,ownership,timestamps -- "$application_path" "$backup_directory/kmainstay"
cp --preserve=mode,ownership,timestamps -- "$initialisation_path" "$backup_directory/kmainstay-initialise"
# Do not glob this prefix: the legacy attachment directory used kmainstay.db.uploads.
database_files=()
for database_file in "$data_directory/kmainstay.db" "$data_directory/kmainstay.db-wal" "$data_directory/kmainstay.db-shm"; do
  [[ -f "$database_file" ]] && database_files+=("$database_file")
done
if [[ ! -f "$data_directory/kmainstay.db" ]]; then
  printf 'database is missing\n' >&2
  exit 1
fi
cp --preserve=mode,ownership,timestamps -- "${database_files[@]}" "$backup_directory/"

rollback_needed=1
rollback() {
  local exit_status=$?
  if (( rollback_needed == 1 )); then
    printf 'deployment failed; restoring previous binary and database\n' >&2
    systemctl stop kmainstay >/dev/null 2>&1 || true
    install -o root -g root -m 0755 "$backup_directory/kmainstay" "$application_path"
    install -o root -g root -m 0755 "$backup_directory/kmainstay-initialise" "$initialisation_path"
    rm -f -- "$data_directory/kmainstay.db" "$data_directory/kmainstay.db-wal" "$data_directory/kmainstay.db-shm"
    restored_database_files=()
    for database_file in "$backup_directory/kmainstay.db" "$backup_directory/kmainstay.db-wal" "$backup_directory/kmainstay.db-shm"; do
      [[ -f "$database_file" ]] && restored_database_files+=("$database_file")
    done
    cp --preserve=mode,ownership,timestamps -- "${restored_database_files[@]}" "$data_directory/"
    systemctl start kmainstay || true
  fi
  rm -f -- "$application_upload" "$initialisation_upload"
  exit "$exit_status"
}
trap rollback EXIT

install -o root -g root -m 0755 -- "$application_upload" "$application_path"
install -o root -g root -m 0755 -- "$initialisation_upload" "$initialisation_path"
systemctl start kmainstay

healthy=0
for _ in {1..20}; do
  if curl --fail --silent --show-error "$health_url" >/dev/null; then
    healthy=1
    break
  fi
  sleep 1
done
if (( healthy == 0 )); then
  printf 'new release failed its health check\n' >&2
  exit 1
fi

installed_application_sha256=$(sha256sum -- "$application_path" | cut -d' ' -f1)
installed_initialisation_sha256=$(sha256sum -- "$initialisation_path" | cut -d' ' -f1)
[[ "$installed_application_sha256" == "$expected_application_sha256" ]]
[[ "$installed_initialisation_sha256" == "$expected_initialisation_sha256" ]]

rollback_needed=0
trap - EXIT
rm -f -- "$application_upload" "$initialisation_upload"
find "$backup_root" -mindepth 1 -maxdepth 1 -type d -printf '%T@ %p\n' \
  | sort -rn \
  | sed -n '11,$p' \
  | cut -d' ' -f2- \
  | xargs -r rm -rf --

printf 'deployment complete\n'
printf 'application_sha256=%s\n' "$installed_application_sha256"
printf 'initialisation_sha256=%s\n' "$installed_initialisation_sha256"
