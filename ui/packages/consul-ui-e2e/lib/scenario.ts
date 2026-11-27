/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { spawn } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { ConsulClient } from './consul-client.ts';
import { waitFor } from './wait.ts';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
export const SCENARIOS_DIR = path.resolve(__dirname, '../scenarios');

export type Runtime = 'docker' | 'local';
export type Edition = 'ce' | 'ent';

export interface ScenarioMeta {
  /** Directory name; must be unique. */
  name: string;
  /** One-line human description. */
  description: string;
  /** UI routes this scenario exercises (for docs / summaries). */
  routes?: string[];
  /** Whether the scenario needs the Connect CA (mesh). Gates readiness. */
  requiresConnect?: boolean;
  /**
   * Whether the scenario's data needs a licensed Consul Enterprise agent
   * (namespaces, admin partitions, ...). Scenarios without it run on either
   * edition. This is a requirement, not a filter: tests choose the edition
   * they apply to with `@ent` / `@ce` tags.
   */
  requiresEnterprise?: boolean;
  /**
   * Whether the scenario only works with `RUNTIME=local` (e.g. `auth-methods`:
   * an OIDC login round-trip needs the browser, the Consul agent, and the IdP
   * to agree on one hostname, which only holds when Consul runs as a local
   * process rather than in its own Docker network — see that scenario's
   * README). Such scenarios always start with the local runtime.
   */
  requiresLocalRuntime?: boolean;
}

/** A scenario's seed module must export `seed(consul)`. */
export interface SeedModule {
  seed(consul: ConsulClient): Promise<void>;
}

export function scenarioDir(name: string): string {
  return path.join(SCENARIOS_DIR, name);
}

export function listScenarios(): string[] {
  return fs
    .readdirSync(SCENARIOS_DIR, { withFileTypes: true })
    .filter((d) => d.isDirectory() && !d.name.startsWith('_'))
    .map((d) => d.name)
    .sort();
}

/**
 * Resolve a CLI arg that may be a scenario name or its 1-based index in
 * `listScenarios()` (the same order `pnpm sc list` prints). Lets you run
 * e.g. `pnpm sc up 2` instead of typing the full name.
 */
export function resolveScenarioName(nameOrIndex: string): string {
  if (!/^\d+$/.test(nameOrIndex)) return nameOrIndex;
  const scenarios = listScenarios();
  const idx = Number(nameOrIndex);
  const name = scenarios[idx - 1];
  if (!name) {
    throw new Error(
      `No scenario #${idx} (there are ${scenarios.length} — run "pnpm sc list" to see numbers).`,
    );
  }
  return name;
}

export function loadMeta(name: string): ScenarioMeta {
  const file = path.join(scenarioDir(name), 'scenario.json');
  if (!fs.existsSync(file)) throw new Error(`Scenario "${name}" not found (missing ${file})`);
  const meta = JSON.parse(fs.readFileSync(file, 'utf8')) as ScenarioMeta;
  return { ...meta, name };
}

// Scenario processes never get the shell's CONSUL_HTTP_* settings, which may
// point at a real cluster (see AGENT_ADDR in lib/consul-client.ts).
function scenarioEnv(): NodeJS.ProcessEnv {
  return Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith('CONSUL_HTTP_')));
}

function run(cmd: string, args: string[], cwd: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const child = spawn(cmd, args, { cwd, stdio: 'inherit', env: scenarioEnv() });
    child.on('error', reject);
    child.on('exit', (code) =>
      code === 0 ? resolve() : reject(new Error(`${cmd} ${args.join(' ')} exited with code ${code}`)),
    );
  });
}

/** The runtime a scenario actually uses: some only support `local`. */
export function runtimeFor(meta: ScenarioMeta, requested: Runtime): Runtime {
  return meta.requiresLocalRuntime ? 'local' : requested;
}

/** Whether anything answers HTTP on the agent address. */
async function agentListening(consul: ConsulClient): Promise<boolean> {
  try {
    await fetch(`${consul.addr}/v1/status/leader`, { signal: AbortSignal.timeout(1_000) });
    return true;
  } catch {
    return false;
  }
}

/**
 * The scenario this package last started and hasn't stopped yet. Written
 * before a start and removed after a stop, so a scenario left running by
 * KEEP_UP=1, `pnpm sc up`, a failed start or a killed run can be told apart
 * from an unrelated process on the agent address.
 */
const ACTIVE_FILE = path.resolve(__dirname, '../.run/active-scenario.json');

interface ActiveScenario {
  name: string;
  runtime: Runtime;
}

export function activeScenario(): ActiveScenario | undefined {
  try {
    return JSON.parse(fs.readFileSync(ACTIVE_FILE, 'utf8')) as ActiveScenario;
  } catch {
    return undefined;
  }
}

function setActiveScenario(active: ActiveScenario | undefined): void {
  if (active) {
    fs.mkdirSync(path.dirname(ACTIVE_FILE), { recursive: true });
    fs.writeFileSync(ACTIVE_FILE, JSON.stringify(active));
  } else {
    fs.rmSync(ACTIVE_FILE, { force: true });
  }
}

/**
 * Start the scenario's Consul agent (docker compose or the local.sh fallback).
 *
 * A scenario this package left running is stopped first. Anything else on the
 * agent address is refused: it would otherwise answer the readiness check and
 * get seeded and tested instead. If the start fails part-way, whatever it did
 * start is stopped again before the error is reported.
 */
export async function startAgent(name: string, runtime: Runtime, consul = new ConsulClient()): Promise<void> {
  const leftover = activeScenario();
  if (leftover) {
    console.log(`[scenario] stopping "${leftover.name}", left running by an earlier run`);
    await stopAgent(leftover.name, leftover.runtime, consul);
  }
  if (await agentListening(consul)) {
    throw new Error(
      `Something not started by this package is serving ${consul.addr}, so "${name}" can't start there. ` +
        'Stop it first (find it with "lsof -i :8500").',
    );
  }

  setActiveScenario({ name, runtime });
  try {
    await runAgentScript(name, runtime, 'up');
  } catch (err) {
    await stopAgent(name, runtime, consul).catch((stopErr: Error) =>
      console.warn(`[scenario] cleanup after failed start of "${name}" also failed: ${stopErr.message}`),
    );
    throw err;
  }
}

/** Tear down the scenario's Consul agent and wait until its address is free. */
export async function stopAgent(name: string, runtime: Runtime, consul = new ConsulClient()): Promise<void> {
  await runAgentScript(name, runtime, 'down');
  await waitFor(async () => !(await agentListening(consul)), {
    timeoutMs: 20_000,
    intervalMs: 250,
    label: `${consul.addr} to stop answering after "${name}" was stopped`,
  });
  if (activeScenario()?.name === name) setActiveScenario(undefined);
}

function runAgentScript(name: string, runtime: Runtime, action: 'up' | 'down'): Promise<void> {
  const dir = scenarioDir(name);
  if (runtime === 'docker') {
    const args = action === 'up' ? ['up', '-d'] : ['down', '-v'];
    return run('docker', ['compose', '-p', `consul-e2e-${name}`, ...args], dir);
  }
  return run('bash', ['local.sh', action], dir);
}

/** Wait for readiness then run the scenario's seed module. */
export async function seedScenario(name: string, consul = new ConsulClient()): Promise<void> {
  const meta = loadMeta(name);
  await consul.waitReady();
  if (meta.requiresConnect) await consul.waitConnectCA();
  if (meta.requiresEnterprise) await consul.waitLicensed();

  const seedPath = path.join(scenarioDir(name), 'seed.ts');
  const mod = (await import(pathToFileURL(seedPath).href)) as SeedModule;
  if (typeof mod.seed !== 'function') throw new Error(`${seedPath} must export a "seed(consul)" function`);
  await mod.seed(consul);
}

/** Convenience: bring the agent up, wait, and seed. */
export async function upScenario(name: string, runtime: Runtime): Promise<void> {
  await startAgent(name, runtime);
  await seedScenario(name);
}

/**
 * With UI_SOURCE=agent the UI under test is whatever the agent binary embeds,
 * so the binary must be chosen explicitly: no `consul` from PATH, no default
 * image. Returns an error message, or undefined if the setup is explicit.
 */
export function agentBinaryProblem(runtime: Runtime): string | undefined {
  if (runtime === 'local') {
    const bin = process.env.CONSUL_BIN ?? '';
    if (!bin.includes('/')) {
      return (
        `UI_SOURCE=agent tests the UI built into the agent binary, so set CONSUL_BIN to the path of the ` +
        `binary under test (got "${bin || 'unset'}", which would use whatever is on PATH).`
      );
    }
  } else if (!process.env.CONSUL_IMAGE) {
    return (
      'UI_SOURCE=agent tests the UI built into the agent image, so set CONSUL_IMAGE to the image under ' +
      'test (e.g. consul:local from `make dev-docker`) instead of the default release image.'
    );
  }
  return undefined;
}

/**
 * Find the UI app bundle the agent serves at `baseURL` and check it kept its
 * `data-test-*` attributes, which production builds strip unless built with
 * KEEP_TEST_SELECTORS=1. Returns the bundle's file name.
 */
export async function checkServedUi(baseURL: string): Promise<string> {
  const indexUrl = new URL('/ui/', baseURL);
  const html = await (await fetch(indexUrl)).text();
  const src = html.match(/src="([^"]*assets\/consul-ui-[0-9a-f]+\.js)"/)?.[1];
  if (!src) throw new Error(`Couldn't find the consul-ui app bundle in ${indexUrl}.`);
  const bundleUrl = new URL(src, indexUrl);
  const bundle = await (await fetch(bundleUrl)).text();
  const name = path.basename(bundleUrl.pathname);
  if (!bundle.includes('data-test-')) {
    throw new Error(
      `The UI served on ${baseURL} (${name}) has no data-test-* attributes: it was built without ` +
        'KEEP_TEST_SELECTORS=1. Rebuild with "KEEP_TEST_SELECTORS=1 make ui-regen && make dev" ' +
        '(see README.md, "Testing a built Consul binary").',
    );
  }
  return name;
}
