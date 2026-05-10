# Release Version Verification

This note documents how release binaries get their version string and how to verify that a published release is showing the expected semantic version instead of a development-style value.

## Source Of Truth

- `webapp/backend/pkg/version/version.go` stores the shared `VERSION` constant used by the web binary, collector banners, and API `server_version`.
- `.releaserc.json` updates that constant during the semantic-release prepare step.
- `.github/workflows/release.yaml` builds binaries from `ref: v${{ needs.release.outputs.new_release_version }}`, which means the release job compiles the tagged release commit after the version constant has been updated.
- `Makefile` passes only `main.goos` and `main.goarch` through `-ldflags`. It does not overwrite `version.VERSION`.

## Verified Paths

Issue `#496` was investigated against the current release flow on May 10, 2026.

- Downloaded `v1.53.0` and `v1.53.1` release assets for:
  - `scrutiny-web-darwin-arm64`
  - `scrutiny-collector-metrics-darwin-arm64`
  - `scrutiny-collector-zfs-darwin-arm64`
  - `scrutiny-collector-performance-darwin-arm64`
- Ran each binary with no arguments and confirmed the startup banner showed the expected semantic version, for example `darwin.arm64-1.53.1`.
- Built the same four binaries locally from the `v1.53.1` tag using:

```bash
GOOS=darwin GOARCH=arm64 go build -buildvcs=false \
  -ldflags '-X main.goos=darwin -X main.goarch=arm64' \
  -o <output> <package>
```

- Confirmed the local tagged builds produced the same semantic-version banner output as the published assets.

## How To Re-Check A Release

1. Download the target asset with `gh release download <tag> -p '<asset-name>' -D <dir>`.
2. Mark it executable with `chmod +x`.
3. Run the binary with no arguments and inspect the banner line.
4. Confirm the suffix matches `<goos>.<goarch>-<semver>`.
5. If needed, build from the release tag locally using the same `-buildvcs=false` and `main.goos` / `main.goarch` ldflags used by the Makefile.

## Notes

- Development builds may still show `dev-<version>` when `goos` and `goarch` are not injected.
- A local `scrutiny start` check may fail before `/api/settings` is reachable if the environment does not provide the required SQLite path or InfluxDB dependency. That startup failure is separate from release-version injection.
