# Production deployment

Live MVP: <https://170-64-239-198.sslip.io>

`sslip.io` supplies temporary IP-derived DNS. Replace it with a domain Michael controls before treating this as a durable production address.

## Server layout

- application: `/opt/kmainstay/kmainstay`
- maintenance CLI: `/opt/kmainstay/kmainstayctl`
- SQLite database: `/var/lib/kmainstay/kmainstay.db`
- systemd unit: `/etc/systemd/system/kmainstay.service`
- Caddy config: `/etc/caddy/Caddyfile`
- SSH policy: `/etc/ssh/sshd_config.d/99-kmainstay.conf`

The application runs as the unprivileged `kmainstay` user and listens only on `127.0.0.1:8080`. Caddy is the only public application edge. UFW allows only SSH, HTTP and HTTPS.

## Deploy an update

Build from a clean, tested checkout:

```sh
npm ci
npm test
npm run build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -race ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o kmainstay ./cmd/kmainstay
```

Copy the binary to a temporary server path, verify its checksum, install it as root-owned mode `0755`, then restart and check:

```sh
systemctl restart kmainstay
systemctl is-active kmainstay
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS https://170-64-239-198.sslip.io/healthz
```

## Data

The SQLite database is the only application state. Back up the database and its WAL consistently before risky changes. No off-server backup destination is configured yet.
