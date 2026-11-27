# AGENTS.md

Instructions for AI coding assistants working in this package
(`ui/packages/consul-ui-e2e`). People should start with [README.md](README.md).

This package holds scenario-based Playwright tests for the Consul UI. Each
scenario under `scenarios/<name>/` boots one Consul agent on `:8500` and seeds
the data its tests in `tests/<name>/` assert on.

## Tasks

When asked to do one of these tasks, open its playbook and follow it step by
step. The playbooks are plain Markdown and don't depend on any AI tool.

- **Add a scenario** (seed data for a UI use case): follow
  [agents/scenario-author.md](agents/scenario-author.md).
- **Add or fix Playwright tests** for a scenario: follow
  [agents/e2e-test-author.md](agents/e2e-test-author.md).

Each playbook tells you which reference doc in `docs/` to read first.

## Commands

Run from this package directory.

- Type-check: `pnpm typecheck`
- List scenarios: `pnpm sc list`
- Start, inspect, stop a scenario: `pnpm sc up <name>`, `pnpm sc status <name>`, `pnpm sc down <name>`
- Run tests (each spec starts the scenario it declares): `pnpm e2e [path]`; Enterprise: `EDITION=ent pnpm e2e`
- Against a built Consul binary instead of the Ember dev server: add
  `UI_SOURCE=agent RUNTIME=local CONSUL_BIN=<path>` (see README.md)

## Rules

- Every spec file declares its scenario at the top level with
  `test.use({ scenario: '<name>' })`. Scenarios must not depend on tests.
- Tag Enterprise-only tests `@ent` and CE-only tests `@ce`; leave tests that
  hold on both editions untagged.
- Prefer `data-test-*` selectors, then accessible roles. Don't assert on HDS
  class names or DOM structure.
- Seed deterministic data only; tests assert on exact values.
- New `.ts`, `.sh`, `.yml` and `.yaml` files need the repository's license
  header. Copy it from an existing file in the package.
- TypeScript here runs on Node's type stripping, so use only erasable syntax:
  no `enum`, `namespace`, or constructor parameter properties.
  `pnpm typecheck` enforces this.
- Never commit license files (`*.hclic`), `.env`, or credentials.
- Verify before finishing: run the scenario or tests you changed, and report
  the result.
