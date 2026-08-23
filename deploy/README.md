# Production deployment

Live MVP: <https://170-64-239-198.sslip.io>

`sslip.io` supplies temporary IP-derived DNS. Replace it with a domain Michael controls before treating this as a durable production address.

## Server layout

- application: `/opt/kmainstay/kmainstay`
- initial setup command: `/opt/kmainstay/kmainstay-initialise`
- SQLite database: `/var/lib/kmainstay/kmainstay.db`
- image attachments: `/var/lib/kmainstay/uploads/`
- systemd unit: `/etc/systemd/system/kmainstay.service`
- Caddy config: `/etc/caddy/Caddyfile`
- SSH policy: `/etc/ssh/sshd_config.d/99-kmainstay.conf`

The application runs as the unprivileged `kmainstay` user and listens only on `127.0.0.1:8080`. Caddy is the only public application edge. UFW allows only SSH, HTTP and HTTPS.

## Deploy an update

Pushing to `main` runs `.github/workflows/deploy-development.yml`. The workflow:

1. installs locked frontend dependencies;
2. runs frontend, reference-bot, Go race and vet checks;
3. rebuilds and verifies the committed embedded frontend;
4. builds stripped Linux AMD64 application and initialisation binaries;
5. uploads them through the restricted `kmainstay-deploy` SSH account;
6. verifies checksums, ensures the private uploads directory exists, backs up the database and current binaries, restarts the service, and checks local and public health.

The root-owned `/usr/local/sbin/kmainstay-install-release` script is installed from `deploy/install-release.sh`. It rolls back the binaries and database when the new release fails its local health check, and keeps the ten newest on-server deployment backups.

GitHub's `development` environment contains only the deployment SSH credential and pinned host keys. Repository variables provide the host and deployment account. The account can upload release files and use `sudo` only for the fixed installation script.

A deployment can also be started manually from **GitHub → Actions → Test and deploy development → Run workflow**. Do not bypass this with a manual binary copy unless repairing the pipeline itself.

## Data

Durable application state consists of the SQLite database and `/var/lib/kmainstay/uploads/`. A valid backup needs a consistent database backup plus the complete uploads directory. The deployment rollback snapshots only the database and binaries; immutable uploads remain in place across rollback. No off-server backup destination is configured yet, so the current deployment is not protected against VPS loss.
