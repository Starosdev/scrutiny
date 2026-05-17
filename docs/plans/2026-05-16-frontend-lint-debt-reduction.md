# Frontend Lint Debt Reduction Plan

**Date:** 2026-05-16
**Status:** In Progress
**Issue:** [#520](https://github.com/Starosdev/scrutiny/issues/520) - chore(frontend): triage and reduce Angular lint debt

## Problem

`npm --prefix webapp/frontend run lint` fails because the Angular frontend has accumulated broad repo-wide lint debt that is not tied to a single feature branch. That makes lint a poor gate for focused frontend work and creates unnecessary review noise when a small issue touches files that already fail unrelated rules.

## Current Snapshot

A fresh ESLint run against `webapp/frontend/src/**/*.{ts,html}` on 2026-05-16 produced `9,881` findings across `223` files:

| Bucket | Findings | Notes |
| ---- | ----: | ---- |
| `prettier/prettier` | 9,482 | Formatting-only churn, `96%` of all findings |
| `@angular-eslint/prefer-inject` | 182 | Constructor injection migration |
| `@angular-eslint/template/prefer-control-flow` | 70 | `*ngIf` / `*ngFor` template migration |
| `@angular-eslint/prefer-standalone` | 51 | Standalone component migration |
| `@angular-eslint/template/eqeqeq` | 40 | Template behavior review required |
| `@angular-eslint/component-selector` | 22 | Selector contract cleanup |
| Other one-off rules | 34 | `no-unused-vars`, `no-useless-escape`, input/output naming, one `ban-ts-comment` |

Largest path clusters from the same run:

- `src/app/data/mock/**`: `5,676` findings across `15` files, almost entirely formatting debt.
- `src/@treo/**`: `1,428` findings across `65` files, mixed formatting and Angular migrations.
- `*.spec.ts`: `1,004` findings across `16` files, overwhelmingly formatting debt.
- `src/app/**/*.html`: `78` findings across `9` files, mostly control-flow and template equality errors.

Representative hot spots:

- `src/app/data/mock/device/details/sdb.ts`: `1,657` findings
- `src/app/data/mock/device/details/sdc.ts`: `1,428`
- `src/app/data/mock/summary/temp_history.ts`: `1,197`
- `src/app/modules/detail/detail.component.ts`: mixed formatting plus `prefer-inject`
- `src/app/layout/common/dashboard-settings/dashboard-settings.component.html`: dense `prefer-control-flow`

## Buckets

### 1. Formatting-only cleanup

Scope:

- `prettier/prettier` findings in TS, HTML, and JSON-backed mock data files

Characteristics:

- Mechanical
- Low review value per line
- No intended product behavior change
- Dominated by mock data, specs, and older `@treo` files

Recommended handling:

- Run `npm --prefix webapp/frontend run format`
- If the diff is too large for one review, split it by path group:
  - `src/app/data/mock/**`
  - `src/**/*.spec.ts`
  - `src/@treo/**` and remaining app sources

Can be fixed independently without product behavior changes: Yes

### 2. Template modernization

Scope:

- `@angular-eslint/template/prefer-control-flow`
- `@angular-eslint/template/eqeqeq`

Characteristics:

- Limited file count
- Touches rendered templates rather than service logic
- `prefer-control-flow` should be mostly syntax migration
- `template/eqeqeq` needs a small behavior review because replacing `==` with `===` can change truthiness or string-number comparisons

Recommended handling:

- Convert `*ngIf` / `*ngFor` to Angular built-in control flow first
- Review each `eqeqeq` change with the bound value types in mind
- Prioritize the densest templates first:
  - `dashboard-settings.component.html`
  - `detail.component.html`
  - `mdadm*.component.html`

Can be fixed independently without product behavior changes: Mostly, but `template/eqeqeq` needs targeted review

### 3. Dependency injection migration

Scope:

- `@angular-eslint/prefer-inject`

Characteristics:

- High error count but narrow rule shape
- Good candidate for Angular schematic-assisted migration
- Mostly TS-only churn in components, directives, and services

Recommended handling:

- Use Angular's inject migration as the baseline refactor
- Review files with many constructor parameters first because they carry the most churn:
  - `src/@treo/components/highlight/highlight.component.ts`
  - `src/app/modules/detail/detail.component.ts`
  - `src/app/modules/dashboard/dashboard.component.ts`
  - layout components under `src/app/layout/**`

Can be fixed independently without intended product behavior changes: Yes, if kept scoped to inject migration only

### 4. Standalone and selector contract cleanup

Scope:

- `@angular-eslint/prefer-standalone`
- `@angular-eslint/component-selector`
- `@angular-eslint/no-input-rename`
- `@angular-eslint/no-output-native`

Characteristics:

- Lower count, higher architectural sensitivity
- Can affect module wiring, selector names, and public template contracts
- Highest chance of churn spilling into unrelated templates and module declarations

Recommended handling:

- Keep this as a dedicated follow-up after formatting, templates, and inject migration are under control
- Decide explicitly whether the repo wants to adopt standalone uniformly or suppress the rule for legacy Treo-era patterns
- Treat selector fixes as contract changes and update call sites in the same change set

Can be fixed independently without product behavior changes: No, this bucket needs deliberate contract review

### 5. One-off TypeScript hygiene

Scope:

- `@typescript-eslint/no-unused-vars`
- `no-useless-escape`
- `@typescript-eslint/ban-ts-comment`

Characteristics:

- Small count
- Best handled last, or folded into the bucket that already touches the same file

Recommended handling:

- Clean these opportunistically once the larger rule families stop dominating the lint output

Can be fixed independently without product behavior changes: Usually yes

## Recommended Execution Order

1. Formatting-only cleanup in isolated reviews because it removes `96%` of the noise and makes later diffs readable.
2. Template modernization because it is localized, visible, and much smaller than the TypeScript migrations.
3. `prefer-inject` migration using a schematic-assisted pass plus focused review.
4. One-off TypeScript hygiene in files already touched by the previous phases.
5. Standalone and selector contract decisions last, because they have the highest chance of widening scope.

## Suggested Issue Split

Use separate follow-up issues or PRs instead of one repo-wide sweep:

1. Prettier formatting cleanup
2. Template control-flow migration
3. Template equality review
4. `prefer-inject` migration
5. Standalone and selector policy decision
6. Residual one-off lint fixes

This keeps mechanical churn separate from semantic Angular migrations and prevents one large PR from mixing formatting, dependency injection, and template contract changes.

## Verification

For each follow-up bucket:

- Run `npm --prefix webapp/frontend run lint`
- Run `npm --prefix webapp/frontend run build` for any TS or template migration
- Run `npm --prefix webapp/frontend test` for template or component refactors that touch behavior-sensitive areas

The plan itself was based on a direct ESLint JSON report generated in the worktree on 2026-05-16 after installing frontend dependencies with `npm --prefix webapp/frontend ci`.

## Progress Update

Formatting cleanup is now complete in this branch.

- Prettier findings reduced from `9,482` to `0`
- Total lint findings reduced from `9,881` to `399`
- Remaining baseline:
  - `182` `@angular-eslint/prefer-inject`
  - `70` `@angular-eslint/template/prefer-control-flow`
  - `51` `@angular-eslint/prefer-standalone`
  - `40` `@angular-eslint/template/eqeqeq`
  - `25` `@typescript-eslint/no-unused-vars`
  - `22` `@angular-eslint/component-selector`
  - `9` other one-off errors

Build verification after the formatting pass completed successfully with `npm --prefix webapp/frontend run build`.
