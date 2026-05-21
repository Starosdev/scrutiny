# Scrutiny API

Scrutiny now documents its HTTP API from a canonical OpenAPI specification:

- OpenAPI spec: [openapi.yaml](./openapi.yaml)
- Swagger UI: [swagger-ui.html](./swagger-ui.html)
- Served Swagger UI path: `/docs/api`
- Served OpenAPI path: `/api/docs/openapi.yaml`
- Default auth behavior: both served docs routes require auth unless `web.docs.public=true`

## Scope

The spec covers the current `/api/*` routes registered in [webapp/backend/pkg/web/server.go](../webapp/backend/pkg/web/server.go).

That includes:

- authentication and session login
- health and diagnostics
- device registration, uploads, details, actions, and performance
- settings, SMART overrides, and notification URLs
- report generation and report history
- filesystem capacity
- ZFS pools
- Btrfs filesystems
- MDADM arrays
- Prometheus metrics

## Auth Model

Scrutiny uses Bearer authentication when `web.auth.enabled` is on.

- Public routes: `/api/health`, `/api/auth/status`, `/api/auth/login`
- Docs routes: `/docs/api` and `/api/docs/openapi.yaml` are protected by default and become public only when `web.docs.public=true`
- Protected routes: all other `/api/*` routes
- Metrics route: `/api/metrics` may accept the general auth token or the dedicated metrics token, depending on configuration

See [AUTH.md](./AUTH.md) for configuration and deployment details.

## Notes

- The OpenAPI document is the source of truth. Do not add new standalone API tables elsewhere in the repo.
- Some collector payloads are intentionally documented as structured objects with representative fields because the backend accepts large collector-origin JSON models.
- Notification URL endpoints cover existing Shoutrrr syntax, explicit `apprise+...` targets, `script://` targets, and raw `http(s)` webhooks.
- If a route is added or changed in `server.go`, update `openapi.yaml` in the same change.
