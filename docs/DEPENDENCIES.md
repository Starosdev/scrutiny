# Dependency Inventory

This document records dependency history, current security checks, and update recommendations. Package manifests and lockfiles remain version source of truth.

Last updated: 2026-09-10

## Table of Contents

- [Summary](#summary)
- [Security Status](#security-status)
- [Go Dependencies](#go-dependencies)
- [NPM Dependencies](#npm-dependencies)
- [Update Strategy](#update-strategy)
- [Related Issues](#related-issues)

---

## Summary

> Version counts below are historical inventory from 2026-03-10. Current security status appears in section below.

| Category | Total | Current | Outdated | Vulnerable |
|----------|-------|---------|----------|------------|
| Go Direct | 24 | 13 | 11 | TBD |
| Go Indirect | ~60 | - | - | TBD |
| NPM Production | 21 | 15 | 6 | 0 |
| NPM Development | 25 | 22 | 3 | 0 |

---

## Security Status

### Frontend tooling fixes (2026-10-21)

The frontend overrides require `follow-redirects` 1.16.1 or later in the 1.x series and Vite 8.3.3 or later in the 8.x series. The lockfile resolves 1.16.1 and 8.3.5 respectively. Angular build tooling 22.2.2 pins Vite 8.3.0, so a lockfile refresh alone cannot replace that vulnerable version. Keep the Vite override until the Angular dependency pins a patched version.

These updates address the credential-header leaks in `follow-redirects` and the editor-launch vulnerabilities bundled in Vite. Both packages are development dependencies. The production Docker image copies compiled frontend assets rather than frontend `node_modules`.

Run `node ../../.github/scripts/audit-release-tooling.mjs` from `webapp/frontend` after dependency updates. The existing audit allowlist remains unchanged. A passing gate can still include the documented, temporary exception for the unpatched `braces` advisory. See issue #1022 and the related `follow-redirects` update in PR #1019.

### Current baseline (2026-09-10)

- Root release tooling: `npm audit --audit-level=low` reports 0 vulnerabilities.
- Frontend dependencies: `npm audit --audit-level=low` reports 0 vulnerabilities.
- Go code: `govulncheck` reports 0 reachable vulnerabilities.
- Go module inventory still includes one uncalled `golang.org/x/crypto/openpgp` finding with no fixed release.
- CI enforces npm and Go checks through `.github/workflows/dependency-audit.yaml`.

### Security cleanup decision record

- **Problem:** Exact transitive dependency overrides stopped moving when later advisories raised fixed versions, and CI had no blocking dependency audit.
- **Approach:** Use security minimums in npm overrides, regenerate both lockfiles, replace legacy `go-util` helpers with tested standard-library code, add blocking npm and Go reachability checks, and require patched Go 1.26.8.
- **Dead ends:** The latest `go-util` release still imports `golang.org/x/crypto/ssh/terminal`. Go 1.25.0 reports patched standard-library findings. The approved Go 1.26 expansion resolves fixed `x/crypto` findings while retaining one uncalled advisory with no fixed release.
- **Rule:** Keep security overrides at patched version floors, run audits against committed lockfiles, and separate uncalled module findings from reachable vulnerabilities.

### Release-tooling audit repair (2026-10-24)

PR #1029 updates the root lockfile to Undici 6.29.1 and 7.30.1, within the
existing dependency ranges. These versions fix the findings in the active
release tooling. The npm override now requires at least 11.21.1, which also
removes the older bundled brace-expansion, http-cache-semantics, and
postcss-selector-parser findings.

The [npm 11.21.1 release](https://github.com/npm/cli/releases/tag/v11.21.1)
still bundles ip-address 10.7.2 and Undici 6.29.0. Root `npm audit` therefore
still reports two vulnerable packages. The audit gate permits only these three
advisories, only under `node_modules/npm/node_modules/`:

- [GHSA-rxp7-6hxx-x8q5](https://github.com/advisories/GHSA-rxp7-6hxx-x8q5): ip-address zone parsing.
- [GHSA-4qv8-2p9j-2685](https://github.com/advisories/GHSA-4qv8-2p9j-2685): Undici EventSource memory use.
- [GHSA-xjj6-f33q-929c](https://github.com/advisories/GHSA-xjj6-f33q-929c): Undici content-encoding error handling.

The bundled npm remains unused because `.releaserc.json` explicitly lists its
plugins and excludes `@semantic-release/npm`. Overrides cannot replace bundled
files. The existing expiry remains November 1, 2026; no exception applies to
active Undici copies outside that bundle. Policy tests enforce the path and
expiry boundaries and the plugin configuration. A passing gate does not mean
that the bundled vulnerabilities are patched. Reassess the exception before
its expiry and whenever the release plugin list changes.

Once the root audit passes, the same CI job reaches the frontend audit. Patch
updates to body-parser 1.20.9 and fast-uri 3.1.9 clear their findings. Angular
CLI 22.2.3 from the target branch brings its pinned Zod dependency to the patched 4.6.6 release.
The existing body-parser and fast-uri override floors match those patches.
No frontend advisory exception is added.

### Release-tooling audit allowlist review (2026-10-30)

Three further Undici advisories reached the bundled npm copy after the previous
review, so root `npm audit` blocked again on the same package and the same path:

- [GHSA-2m5v-pch6-74hx](https://github.com/advisories/GHSA-2m5v-pch6-74hx)
- [GHSA-qrxm-fqhp-jwvm](https://github.com/advisories/GHSA-qrxm-fqhp-jwvm)
- [GHSA-9fm3-97px-rjf6](https://github.com/advisories/GHSA-9fm3-97px-rjf6)

A newer npm does not fix them. npm 12.2.1, the latest release on this date,
bundles ip-address 10.7.2 and Undici 6.29.0, the same versions npm 11.21.1
bundles. This was measured rather than assumed: raising the npm override to
`^12.2.1`, regenerating the root lockfile with `npm install --package-lock-only`
and re-running the audit gate reported the identical two packages, so the
override was reverted and the lockfile left unchanged. Overrides cannot replace
files shipped inside a package tarball.

The three advisories are added to the allowlist on the existing terms: every
affected path stays under `node_modules/npm/node_modules/`, and `.releaserc.json`
loads six plugins, none of which is `@semantic-release/npm`, so the bundled npm
never executes.

The fix is close, not distant. npm/cli's default branch already bundles Undici
6.29.1 and ip-address 10.7.5, which patch every advisory listed here, and npm
ships every few weeks. The expiry therefore moves only to December 15, 2026.
Removing the exception then needs the `npm` override in the root `package.json`
raised past the 11.x floor, because the fix lands in npm 12.x; that override is
the knob, even though it cannot reach the nested copies directly.

One of the three changes the risk class of this exception.
[GHSA-2m5v-pch6-74hx](https://github.com/advisories/GHSA-2m5v-pch6-74hx) is a TLS
certificate validation bypass, where every earlier entry was a denial of service.
The exception still rests on the bundled npm never executing, but the cost of
that argument being wrong is now confidentiality rather than availability, which
is the reason for the short expiry rather than a longer one.

A passing gate still does not mean the bundled copies are patched.

The frontend audit, which only runs once the root audit passes, reported one
finding: qs 6.16.0, [GHSA-6pq8-jhm2-2vcw](https://github.com/advisories/GHSA-6pq8-jhm2-2vcw).
A lockfile-only update to qs 6.17.1 clears it. The body-parser, fast-uri and Zod
findings from the previous review are already resolved and no longer appear.

The `braces` entry in `UNPATCHED_ADVISORIES`
([GHSA-vfj7-8cjw-p6xm](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm)) is
removed. GitHub withdrew that advisory on October 23, 2026, and neither audit
reports the package, so the exception covered nothing and the shared expiry had
been carrying it forward. `UNPATCHED_ADVISORIES` is now empty.

### glob and minimatch advisories (2026-10-30)

GitHub published two advisories on 2026-10-30 that broke the audit gate on every
branch at once:

- [GHSA-3g3m-2fjr-6pw4](https://github.com/advisories/GHSA-3g3m-2fjr-6pw4):
  glob AbortSignal cancellation bypass. Affects `>= 9.0.0, <= 14.0.1`, patched in
  14.0.2.
- [GHSA-g3hv-75wg-m249](https://github.com/advisories/GHSA-g3hv-75wg-m249):
  minimatch denial of service through globparts amplification. Affects
  `< 11.0.1`, patched in 11.0.1.

Both patches are major releases published on 2026-10-27, and both packages reach
this repository only as transitive dependencies. The gate now carries two
separate exceptions for them.

**Release tooling: bundled npm.** Root `npm audit` reports each package at one
path, `node_modules/npm/node_modules/glob` and
`node_modules/npm/node_modules/minimatch`. npm 11.21.1, the override floor, and
npm 12.2.1, the latest release, both declare `glob ^13.0.6` and
`minimatch ^10.2.5`, so neither range can reach a patched version and bundled
files cannot be replaced by an override. Both advisories are therefore added to
the existing bundled-npm allowlist, which applies only while every reported path
stays inside that tree. Removing them needs an npm release that bumps those two
ranges.

**Frontend: dev-only exception.** The frontend reports minimatch at
`node_modules/minimatch` (3.1.5) and
`node_modules/@typescript-eslint/typescript-estree/node_modules/minimatch`
(10.2.6), and 15 further packages flagged only through it: eslint,
typescript-eslint, angular-eslint, karma, karma-coverage, glob, rimraf and their
dependencies. No consumer in the committed lockfile supports minimatch 11:
eslint 9.39.5, `@eslint/eslintrc` 3.3.6 and `@eslint/config-array` 0.21.2 declare
`minimatch ^3.1.5`, karma 6.4.4 and karma-coverage 2.2.1 declare `^3.0.4`, and
`@typescript-eslint/typescript-estree` 8.72.0 declares `^10.2.2`. Registry-latest
is no better: eslint 10.14.0 declares `^10.2.6` and typescript-estree 8.72.1
declares `^10.2.2`, so upgrading the tooling would not help either. minimatch 11
no longer exports the v3 call style the karma chain uses, so forcing the major
through an override would break the test runner without fixing the finding.

Every flagged path is build and test tooling, and `npm audit --omit=dev` on the
frontend reports no findings. That is the condition the new exception enforces:
`DEV_ONLY_ADVISORIES` in
[audit-release-tooling.mjs](../.github/scripts/audit-release-tooling.mjs) names
one package per advisory, and the script runs a second production-only audit to
decide dev-only status rather than taking it on trust. A package that becomes a
production dependency loses the exception without anyone editing the allowlist.
The residual exposure is a CI job processing its own repository's glob patterns.

The exception is capped at moderate severity. Unlike the bundled npm, build and
test tooling does execute, in CI, with repository credentials, which is why this
gate audits devDependencies at all instead of running `npm audit --omit=dev`. A
high or critical advisory in dev tooling therefore still fails the gate and gets
an explicit decision rather than an allowlist line. The exception also fails
closed if the production audit returns no vulnerabilities object, so a malformed
second audit cannot mark the whole tree dev-only.

The exception shares the existing `2026-12-15` expiry, which is when the
ecosystem gets re-checked for a non-breaking upgrade path. Policy tests cover the
production-tree boundary, the package boundary, the severity cap, the expiry, the
fail-closed path, and that a bundled-only advisory cannot reach the dev-only
branch.

One unrelated frontend finding published in the same window is fixed by raising
an override floor, with no exception: follow-redirects to 1.16.2 for
[GHSA-mrwp-f3q8-3g9p](https://github.com/advisories/GHSA-mrwp-f3q8-3g9p) (high),
which the regenerated lockfile resolves from 1.16.1 to 1.16.2.

The qs floor is raised from `^6.15.4` to `^6.16.1` in the same commit for
[GHSA-6pq8-jhm2-2vcw](https://github.com/advisories/GHSA-6pq8-jhm2-2vcw)
(moderate, affects `>= 6.7.0, <= 6.16.0`). That one cleared no finding: the
lockfile already resolved qs to 6.17.1 and the installed version does not change.
The raise only stops a future resolution from falling back below the patch.

### Known Reachable Vulnerabilities

No known reachable vulnerabilities remain in current npm audit or Go vulnerability scans.

### Historical Vulnerabilities

| Package | Type | Severity | CVE/Advisory | Status |
|---------|------|----------|--------------|--------|
| quill | npm | Moderate | GHSA-4943-9vgg-gr5r (XSS) | Resolved by removal; retained for history |

### History

#### Fixed/Updated (2026-01-08)

| Package | Severity | Fix Applied |
|---------|----------|-------------|
| @modelcontextprotocol/sdk | High | npm audit fix |
| qs | High | npm audit fix |
| semver | High | @angular-eslint update |
| js-yaml | Moderate | @angular-eslint update |

#### Updated (2026-03-10)

Go direct dependencies updated as part of feature development:

| Package | Old | New | Reason |
|---------|-----|-----|--------|
| fatih/color | v1.15.0 | v1.18.0 | Routine update |
| nicholas-fedor/shoutrrr | v0.8.17 | v0.13.2 | Major update, new features |
| sirupsen/logrus | v1.6.0 | v1.8.3 | Routine update |
| spf13/viper | v1.15.0 | v1.21.0 | Routine update |
| stretchr/testify | v1.8.1 | v1.11.1 | Routine update |
| golang.org/x/sync | v0.3.0 | v0.19.0 | Routine update |

New Go direct dependencies added:

| Package | Version | Added For |
|---------|---------|-----------|
| github.com/eclipse/paho.mqtt.golang | v1.5.0 | Home Assistant MQTT Discovery |
| github.com/go-pdf/fpdf | v0.9.0 | Scheduled reports PDF export |
| github.com/go-viper/mapstructure/v2 | v2.5.0 | Successor to mitchellh/mapstructure |
| github.com/golang-jwt/jwt/v5 | v5.3.1 | API authentication |
| github.com/google/uuid | v1.3.0 | UUID generation |
| go.uber.org/automaxprocs | v1.6.0 | Auto GOMAXPROCS tuning |

---

## Go Dependencies

### Go Version

- **Current floor**: 1.26.8
- **Recommended**: Current

### Direct Dependencies

| Package | Version | Latest | Gap | Priority |
|---------|---------|--------|-----|----------|
| github.com/analogj/go-util | Removed | n/a | Replaced 2026-09-10 | - |
| github.com/eclipse/paho.mqtt.golang | v1.5.0 | v1.5.0 | Current | - |
| github.com/fatih/color | v1.18.0 | v1.18.0 | Current | - |
| github.com/gin-gonic/gin | v1.9.1 | v1.11.0 | 2 minor | High |
| github.com/glebarez/sqlite | v1.4.5 | v1.11.0 | 6 minor | Medium |
| github.com/go-gormigrate/gormigrate/v2 | v2.0.0 | v2.1.5 | 1 minor | Low |
| github.com/go-pdf/fpdf | v0.9.0 | v0.9.0 | Current | - |
| github.com/go-viper/mapstructure/v2 | v2.5.0 | v2.5.0 | Current | - |
| github.com/golang-jwt/jwt/v5 | v5.3.1 | v5.3.1 | Current | - |
| github.com/golang/mock | v1.6.0 | v1.6.0 | Current | - |
| github.com/google/uuid | v1.3.0 | v1.6.0 | 3 minor | Low |
| github.com/influxdata/influxdb-client-go/v2 | v2.9.0 | v2.14.0 | 5 minor | Medium |
| github.com/jaypipes/ghw | v0.6.1 | v0.21.2 | 15 minor | High |
| github.com/mitchellh/mapstructure | v1.5.0 | v1.5.0 | Current | - |
| github.com/nicholas-fedor/shoutrrr | v0.13.2 | v0.13.2 | Current | - |
| github.com/prometheus/client_golang | v1.17.0 | v1.23.2 | 6 minor | Medium |
| github.com/samber/lo | v1.25.0 | v1.52.0 | 27 minor | Medium |
| github.com/sirupsen/logrus | v1.8.3 | v1.9.3 | 1 minor | Low |
| github.com/spf13/viper | v1.21.0 | v1.21.0 | Current | - |
| github.com/stretchr/testify | v1.11.1 | v1.11.1 | Current | - |
| github.com/urfave/cli/v2 | v2.2.0 | v2.27.7 | 25 minor | High |
| go.uber.org/automaxprocs | v1.6.0 | v1.6.0 | Current | - |
| golang.org/x/sync | v0.19.0 | v0.19.0 | Current | - |
| gorm.io/gorm | v1.23.5 | v1.31.1 | 8 minor | High |

### Deprecated Indirect Dependencies

| Package | Status | Replacement |
|---------|--------|-------------|
| github.com/golang/protobuf | Deprecated | google.golang.org/protobuf |
| github.com/deepmap/oapi-codegen | Deprecated | Consider alternatives |
| github.com/cncf/udpa/go | Deprecated | No longer maintained |

---

## NPM Dependencies

### Angular Framework

| Package | Version | Status |
|---------|---------|--------|
| @angular/core | ^22.1.2 | Current |
| @angular/common | ^22.1.2 | Current |
| @angular/compiler | ^22.1.2 | Current |
| @angular/forms | ^22.1.2 | Current |
| @angular/router | ^22.1.2 | Current |
| @angular/animations | ^22.1.2 | Current |
| @angular/platform-browser | ^22.1.2 | Current |
| @angular/platform-browser-dynamic | ^22.1.2 | Current |

### Angular Material

| Package | Version | Expected | Status |
|---------|---------|----------|--------|
| @angular/material | ^22.1.2 | ^22.x | Current |
| @angular/cdk | ^22.1.2 | ^22.x | Current |

### Production Dependencies

| Package | Version | Status | Notes |
|---------|---------|--------|-------|
| crypto-js | ^4.1.1 | Current | |
| highlight.js | ^11.6.0 | Current | |
| humanize-duration | ^3.27.3 | Current | |
| lodash | ^4.18.1 | Current | |
| marked | ^17.0.1 | Current | |
| dayjs | ^1.11.20 | Current | Replaced moment |
| ng-apexcharts | ^2.4.0 | Current | |
| perfect-scrollbar | ^1.5.5 | Current | |
| rrule | ^2.7.1 | Current | |
| rxjs | ^7.5.7 | Current | |
| tslib | ^2.4.1 | Current | |
| web-animations-js | ^2.3.2 | Current | |
| zone.js | ^0.15.1 | Current | |

### Development Dependencies

| Package | Version | Status |
|---------|---------|--------|
| @angular/cli | ^22.1.4 | Current |
| @angular/build | ^22.1.4 | Current |
| @angular/compiler-cli | ^22.1.2 | Current |
| @angular/language-service | ^22.1.2 | Current |
| @angular-eslint/builder | ^22.1.0 | Current |
| @angular-eslint/eslint-plugin | ^22.1.0 | Current |
| @angular-eslint/eslint-plugin-template | ^22.1.0 | Current |
| @angular-eslint/template-parser | ^22.1.0 | Current |
| @angular-eslint/schematics | ^22.1.0 | Current |
| @typescript-eslint/eslint-plugin | ^8.67.0 | Current |
| @typescript-eslint/parser | ^8.67.0 | Current |
| apexcharts | ~3.35.0 | Outdated | |
| eslint | ^8.57.1 | Current | |
| eslint-config-prettier | ^8.10.2 | Current | |
| eslint-plugin-prettier | ^4.2.5 | Current | |
| jasmine-core | ^4.5.0 | Current | |
| jasmine-spec-reporter | ^7.0.0 | Current | |
| karma | ^6.4.1 | Current | |
| karma-chrome-launcher | ^3.1.1 | Current | |
| karma-coverage | ^2.2.0 | Current | |
| karma-jasmine | ^5.1.0 | Current | |
| karma-jasmine-html-reporter | ^2.0.0 | Current | |
| ngx-markdown | ^21.0.1 | Current | |
| prettier | ^2.8.8 | Current | |
| tailwindcss | ^3.2.3 | Outdated | v4.x available |
| ts-node | ^10.9.1 | Current | |
| typescript | ~5.9 | Current | |

---

## Update Strategy

### Immediate (2026-09-10)

- [x] Remediate npm audit findings in root and frontend lockfiles.
- [x] Add recurring npm audit and Go vulnerability gates.
- [x] Raise Go security floor to 1.26.8 and update `golang.org/x/crypto` to v0.56.0.

### Phase 2: Low-Risk Go Updates (Complete)

The following were updated as part of feature development (2026-03-10):
- fatih/color v1.15.0 → v1.18.0
- sirupsen/logrus v1.6.0 → v1.8.3
- stretchr/testify v1.8.1 → v1.11.1
- golang.org/x/sync v0.3.0 → v0.19.0
- spf13/viper v1.15.0 → v1.21.0
- nicholas-fedor/shoutrrr v0.8.17 → v0.13.2

### Phase 3: Angular Material Alignment (Complete)

```bash
ng update @angular/material @angular/cdk
```

### Phase 4: Medium-Risk Go Updates

```bash
go get -u github.com/samber/lo@latest
go get -u github.com/prometheus/client_golang@latest
go get -u github.com/influxdata/influxdb-client-go/v2@latest
go get -u github.com/spf13/viper@latest
```

### Phase 5: High-Risk Go Updates

```bash
go get -u github.com/gin-gonic/gin@latest
go get -u github.com/urfave/cli/v2@latest
go get -u github.com/jaypipes/ghw@latest
go get -u gorm.io/gorm@latest
```

---

## Related Issues

- GitHub #36: Dependency Health Check (this audit)
- GitHub #69: Quill XSS concern (resolved by dependency removal)
- GitHub #70: moment.js migration (resolved by dayjs)
- Angular Material v22 alignment: Complete
- Phase 2 Go updates: Complete (2026-03-10)
- Phase 3 Angular Material: Pending
- Phase 4 medium-risk Go updates: Pending
- Phase 5 high-risk Go updates: Pending

---

## Maintenance

To check for vulnerabilities:

```bash
# NPM
cd webapp/frontend
npm audit

# Go
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
```

To check for outdated packages:

```bash
# NPM
cd webapp/frontend
npm outdated

# Go
go list -m -u all
```
