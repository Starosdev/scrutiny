# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`CONTEXT.md`** at the repo root if it exists
- **`docs/adr/`** if it exists and contains decisions relevant to the area being changed

If these files do not exist, proceed silently. Do not block work or propose creating them unless the current task specifically calls for domain-doc setup.

## File structure

This repo is configured as a single-context repo:

```text
/
├── CONTEXT.md
├── docs/adr/
└── collector/
└── webapp/
```

## Use the project's vocabulary

When naming domain concepts in issues, plans, tests, or refactor proposals, prefer terms already established in the repo's docs and code.

## Flag ADR conflicts

If a proposed change contradicts an existing ADR, call that out explicitly instead of silently overriding it.
