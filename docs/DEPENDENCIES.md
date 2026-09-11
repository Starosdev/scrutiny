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

### Current baseline (2026-09-10)

- Root release tooling: `npm audit --audit-level=low` reports 0 vulnerabilities.
- Frontend dependencies: `npm audit --audit-level=low` reports 0 vulnerabilities.
- Go code: `govulncheck` reports 0 reachable vulnerabilities.
- Go module inventory still includes three uncalled `golang.org/x/crypto` findings. Two are fixed in v0.56.0, which requires Go 1.26. One has no fixed release.
- CI enforces npm and Go checks through `.github/workflows/dependency-audit.yaml`.

### Security cleanup decision record

- **Problem:** Exact transitive dependency overrides stopped moving when later advisories raised fixed versions, and CI had no blocking dependency audit.
- **Approach:** Use security minimums in npm overrides, regenerate both lockfiles, add blocking npm and Go reachability checks, and require a patched Go 1.25 toolchain.
- **Dead ends:** `golang.org/x/crypto@v0.56.0` requires Go 1.26. `govulncheck@v1.8.0` also requires Go 1.26. Go 1.25.0 reports patched standard-library findings. Do not select those paths without an approved toolchain expansion.
- **Rule:** Keep security overrides at patched version floors, run audits against committed lockfiles, and separate uncalled module findings from reachable vulnerabilities.

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

- **Current floor**: 1.25.14
- **Recommended**: Current

### Direct Dependencies

| Package | Version | Latest | Gap | Priority |
|---------|---------|--------|-----|----------|
| github.com/analogj/go-util | v0.0.0-20190301 | v0.0.0-20210417 | 2 years | Low |
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
- [x] Raise Go security floor to 1.25.14.

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
go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...
```

To check for outdated packages:

```bash
# NPM
cd webapp/frontend
npm outdated

# Go
go list -m -u all
```
