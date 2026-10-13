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

TODO (WP-B, SEC-01 / BRK-01): authentication is on by default. Document how the token is
generated (omnibus) or must be supplied (hub and spoke), where collectors read it, and that
the frozen `:1` tag is the opt-out. Link `docs/AUTH.md`.

## `script://` notification URLs are config-file only

TODO (WP-B, SEC-02): `script://` URLs can no longer be added through the UI or API. Describe
how to re-add existing ones in the config file, and what happens to ones already stored in
the database.

## Generated InfluxDB credentials

TODO (docker work package, SEC-03): the shipped well-known InfluxDB admin token and password
are replaced by credentials generated at first setup, and port 8086 is no longer published in
the examples. Describe how existing installs keep working and how to rotate.

## Config schema cleanup

TODO (WP-B, BRK-04): list every renamed, removed, or newly required config key, including the
`notify.filter_attributes` and `notify.level` removal, and where each moved setting now lives.
Point to the regenerated `example.scrutiny.yaml` and `example.collector.yaml`.

## Other operator-visible changes

TODO (owning work packages): volume ownership changes for the `web` container, if any, and
anything else the 2.0 breaking-change list adds.
