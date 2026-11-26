#!/usr/bin/env node
/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { ConsulClient } from '../lib/consul-client.ts';
import {
  listScenarios,
  loadMeta,
  resolveScenarioName,
  runtimeFor,
  seedScenario,
  startAgent,
  stopAgent,
  type Runtime,
} from '../lib/scenario.ts';
import { loadDotEnv } from '../lib/env.ts';

loadDotEnv();

function parseRuntime(args: string[]): Runtime {
  if (args.includes('--local')) return 'local';
  if (args.includes('--docker')) return 'docker';
  return (process.env.RUNTIME as Runtime) ?? 'docker';
}

const USAGE = `Usage:
  pnpm sc up     <name|#> [--local|--docker]   boot + wait ready + seed
  pnpm sc seed   <name|#>                       (re-)seed a running agent
  pnpm sc down   <name|#> [--local|--docker]    tear down
  pnpm sc status <name|#>                       show agent + seeded data
  pnpm sc list                                  list available scenarios (with numbers)

<name|#> accepts either the scenario name or its number from "pnpm sc list",
e.g. "pnpm sc up 2". ("pnpm scenario" also works, "sc" is just the short alias)`;

async function main(): Promise<void> {
  const [command, rawName, ...rest] = process.argv.slice(2);
  const runtime = parseRuntime(rest);

  if (!command || command === 'help' || command === '--help') {
    console.log(USAGE);
    return;
  }

  if (command === 'list') {
    listScenarios().forEach((s, i) => {
      const meta = loadMeta(s);
      const ent = meta.requiresEnterprise ? 'ENT' : '   ';
      console.log(`  ${String(i + 1).padStart(2)}  ${s.padEnd(22)} ${ent}  ${meta.description}`);
    });
    return;
  }

  if (!rawName) throw new Error(`"${command}" needs a scenario name or number.\n\n${USAGE}`);
  const name = resolveScenarioName(rawName);

  switch (command) {
    case 'up': {
      const meta = loadMeta(name);
      const scenarioRuntime = runtimeFor(meta, runtime);
      console.log(`▶ Bringing up "${name}" (${scenarioRuntime}) — ${meta.description}`);
      if (meta.requiresEnterprise) {
        console.log(
          scenarioRuntime === 'docker'
            ? '  (Enterprise scenario — needs an Enterprise CONSUL_IMAGE + CONSUL_LICENSE, see scenario README)'
            : '  (Enterprise scenario — needs an Enterprise CONSUL_BIN + CONSUL_LICENSE_PATH, see scenario README)',
        );
      }
      await startAgent(name, scenarioRuntime);
      await seedScenario(name);
      await printStatus(name);
      break;
    }
    case 'seed': {
      console.log(`▶ Seeding "${name}"`);
      await seedScenario(name);
      await printStatus(name);
      break;
    }
    case 'down': {
      const scenarioRuntime = runtimeFor(loadMeta(name), runtime);
      console.log(`▶ Tearing down "${name}" (${scenarioRuntime})`);
      await stopAgent(name, scenarioRuntime);
      break;
    }
    case 'status':
      await printStatus(name);
      break;
    default:
      throw new Error(`Unknown command "${command}".\n\n${USAGE}`);
  }
}

async function printStatus(name: string): Promise<void> {
  const meta = loadMeta(name);
  const consul = new ConsulClient();
  try {
    const node = await consul.nodeName();
    const services = Object.keys(await consul.services());
    const nodes = (await consul.catalogNodes()).length;
    console.log(`\n  Scenario : ${name} — ${meta.description}`);
    console.log(`  Agent    : ${consul.addr}  (node: ${node})`);
    console.log(`  Nodes    : ${nodes}`);
    console.log(`  Services : ${services.join(', ') || '(none)'}`);
    const uiBase = process.env.UI_SOURCE === 'agent' ? consul.addr : 'http://localhost:4200';
    for (const route of meta.routes ?? []) console.log(`  UI       : ${uiBase}${route}`);
  } catch (err) {
    console.log(`  (agent not reachable at ${consul.addr}: ${(err as Error).message})`);
  }
}

main().catch((err) => {
  console.error(`✖ ${(err as Error).message}`);
  process.exit(1);
});
