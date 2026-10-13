# Upgrading to Scrutiny 2.0

Status: skeleton. Each section names the work package that owns it. Replace each TODO with
the final behaviour, the exact setting names, and the steps an operator must take before the
2.0.0 release is announced. The 2.0.0 release commit must carry a real `BREAKING CHANGE:`
footer for every item below.

## Opt-out for the whole release: the `:1` image tag

TODO (WP-F): confirm a `:1` tag is published for every image in `docker/image-matrix.json`
(see the `{{major}}` patterns in `docker-build.yaml`) before announcing 2.0. Operators who
cannot adopt 2.0 yet stay on the frozen `:1` tag.

## Minimum upgrade-from version

TODO (database work package): the migration baseline squash (BRK-02) declares a minimum
version that databases must already be on. Proposed: 1.70.0. Databases older than that must
step through the latest 1.x release first. Confirm the number when the baseline lands.

## Authentication is enabled by default

`web.auth.enabled` now defaults to `true`. Every API endpoint except `/api/health`, `/api/auth/status`, and `/api/auth/login` requires a Bearer token. Full reference: [AUTH.md](AUTH.md).

| Deployment | What happens at first start | What you do |
|---|---|---|
| Omnibus (`latest-omnibus`) | If `web.auth.token` is empty, the server generates a token and saves it to `/opt/scrutiny/config/.auth-token` (mode 0600). The collectors in the container read that file. The log names the file and never prints the token. | Read the token with `docker exec <container> cat /opt/scrutiny/config/.auth-token` and use it on the login page. The config volume must be writable. |
| Hub/spoke (`latest-web` plus collectors) | The web container exits with an error naming `web.auth.token`. | Set `SCRUTINY_WEB_AUTH_TOKEN` on the web container and `COLLECTOR_API_TOKEN` to the same value on every collector. |
| Already using `web.auth.*` | Unchanged. | Nothing. |

The JWT signing secret is also saved (`/opt/scrutiny/config/.jwt-secret`) when `web.auth.jwt_secret` is empty, so logged-in sessions survive restarts.

To run without authentication, set `SCRUTINY_WEB_AUTH_ENABLED=false` (or `web.auth.enabled: false`). The server logs a warning at every start. Operators who cannot adopt 2.0 yet stay on the frozen `:1` tag.

If you set your own `SCRUTINY_WEB_AUTH_TOKEN` on an omnibus container, also set `COLLECTOR_API_TOKEN` to the same value.

Related changes in the same release:

- `web.listen.trusted_proxies` (new, default empty): the login rate limiter no longer trusts `X-Forwarded-For` from arbitrary clients. Behind a reverse proxy, list the proxy's IP or CIDR here; until you do, logs and the rate limiter see the proxy address.
- `web.listen.max_body_bytes` (new, default 16 MiB): larger request bodies get HTTP 413.
- `web.secrets.dir` (new, default `/opt/scrutiny/config`): where generated secrets are saved.

## `script://` notification URLs are config-file only

`script://` notification URLs can no longer be added through the UI or API. `POST /api/settings/notify-urls` returns HTTP 400 for them, and the per-URL test endpoint refuses a stored one. A script URL runs a program on the host, so it is now accepted only from the `notify.urls` setting in `scrutiny.yaml` (or `SCRUTINY_NOTIFY_URLS`).

Script URLs already stored in the database are skipped when notifications are sent. The server logs a warning each time it skips one, and the row is left in place so you can read the value. To keep using a script target:

1. Open Settings, Notification URLs, and note the stored `script://` URL.
2. Add it to `notify.urls` in `scrutiny.yaml`.
3. Delete the stored entry.

## Report PDF directory is config-only

The `report_pdf_path` dashboard setting is removed from the settings API. PDF reports are written to `web.reports.path` (default `/opt/scrutiny/reports`, env `SCRUTINY_WEB_REPORTS_PATH`). If your database holds a custom `report_pdf_path`, it is ignored and the server logs one warning naming the new setting. `GET /api/reports/history` no longer returns a `path` field (server filesystem paths); it returns `name`, `size`, and `mod_time`.

## Generated InfluxDB credentials

TODO (docker work package, SEC-03): the shipped well-known InfluxDB admin token and password
are replaced by credentials generated at first setup, and port 8086 is no longer published in
the examples. Describe how existing installs keep working and how to rotate.

## Config schema cleanup

Added:

- `web.auth.enabled` default changed from `false` to `true` (see above).
- `web.secrets.dir`, `web.listen.trusted_proxies`, `web.listen.max_body_bytes`, `web.reports.path`.

Removed:

- The startup errors for `notify.filter_attributes` and `notify.level`. These keys were already unsupported (they failed validation with "deprecated"). They are now silently ignored; delete them from `scrutiny.yaml`. Their replacements are the notification level and status threshold settings in the dashboard.
- The `report_pdf_path` setting (see above).
- The startup check that `web.auth.token` is non-empty whenever auth is enabled moved from config validation to server startup, where the omnibus image can generate one. Hub/spoke behavior (refuse to start) is unchanged in effect.

TODO (WP-F): regenerate `example.scrutiny.yaml` and `example.collector.yaml` from `Init()` defaults with a drift test. This package updated the auth, listen, secrets, and reports entries by hand.

## API contract and scoped tokens

Added, with no breaking change:

- `GET /api/version` is a new public endpoint. It returns the server version, `api_version` (`"2"`), and `auth_required` to anyone, plus a `capabilities` object for authenticated requests (`auth_required`, `scoped_tokens`, `collector_trigger_enabled`, `zfs_pool_modifications_allowed`, `metrics_enabled`, `mqtt_enabled`). `docs/openapi.yaml` `info.version` is now `2` instead of `1.0.0`. `server_version`, `collector_trigger_enabled`, and `zfs_pool_modifications_allowed` stay in the `GET`/`POST /api/settings` responses for existing clients.
- Named API tokens with `read`, `collector`, and `admin` scopes: `POST`, `GET`, and `DELETE /api/settings/tokens`. See [AUTH.md](AUTH.md#named-api-tokens-and-scopes).
- Routes are now grouped by minimum scope. The master token, session JWTs, and auth-disabled mode behave as before (full access), so no existing client changes. The only way to get a 403 for scope is to use a new named token above its scope.
- A Go test now fails when a registered route and `docs/openapi.yaml` disagree.

Follow-up outside this change: the settings page needs a token management UI, and the frontend can read the capability flags from `/api/version` instead of copying them from the settings response.

## Other operator-visible changes

TODO (owning work packages): volume ownership changes for the `web` container, if any, and
anything else the 2.0 breaking-change list adds.
