# Scenario: _template

Copy this directory to `scenarios/<name>/` and fill in:

- `scenario.json` — description, UI `routes`, and `requiresConnect` if the
  use-case needs the service mesh / Connect CA.
- `docker-compose.yml` — the Consul topology (usually the single dev agent as-is).
- `seed.ts` — export `seed(consul)` that registers the data.
- `local.sh` — usually unchanged; the local-binary fallback runner.
- This `README.md` — what the scenario sets up and which UI it targets.

Then: `pnpm sc up <name>` and write specs under `tests/<name>/`.
See [`docs/adding-a-scenario.md`](../../docs/adding-a-scenario.md).
