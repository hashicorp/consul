/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import fs from 'node:fs';
import path from 'node:path';
import type { FullResult, Reporter, TestCase, TestError } from '@playwright/test/reporter';

/**
 * Writes a Markdown summary of the run to the GitHub Actions job summary
 * (`$GITHUB_STEP_SUMMARY`), so results show on the run page without
 * downloading the HTML report: totals, the Consul build and UI bundle each
 * scenario ran against, per-file results, and each failure with its error.
 * Does nothing outside GitHub Actions.
 */
export default class GitHubSummaryReporter implements Reporter {
  private tests = new Set<TestCase>();
  private scenarioLines = new Set<string>();

  onTestEnd(test: TestCase): void {
    this.tests.add(test);
  }

  // The scenario fixture logs `[scenario] "<name>" on Consul <version> (<rev>), UI: <bundle>`.
  onStdOut(chunk: string | Buffer): void {
    for (const line of String(chunk).split('\n')) {
      if (/^\[scenario\] ".+" on Consul /.test(line)) this.scenarioLines.add(line.trim());
    }
  }

  onEnd(result: FullResult): void {
    const file = process.env.GITHUB_STEP_SUMMARY;
    if (!file) return;
    fs.appendFileSync(file, this.markdown(result));
  }

  private markdown(result: FullResult): string {
    const tests = [...this.tests];
    const count = (outcome: ReturnType<TestCase['outcome']>) => tests.filter((t) => t.outcome() === outcome).length;
    const failed = count('unexpected');
    const flaky = count('flaky');
    const totals = [
      `${count('expected')} passed`,
      failed && `${failed} failed`,
      flaky && `${flaky} flaky`,
      count('skipped') && `${count('skipped')} skipped`,
    ].filter(Boolean);
    const edition = (process.env.EDITION ?? 'ce').toUpperCase();
    const icon = result.status === 'passed' ? '✅' : '❌';
    const out: string[] = [
      `### ${icon} Scenario e2e tests (${edition}): ${totals.join(', ')} in ${seconds(result.duration)}`,
      '',
    ];
    if (result.status !== 'passed' && !failed) out.push(`Run status: **${result.status}**`, '');

    const scenarios = [...this.scenarioLines].map((line) =>
      line.match(/^\[scenario\] "([^"]+)" on Consul (\S+)(?: \(([^)]+)\))?, UI: (.+)$/),
    );
    if (scenarios.length) {
      out.push('| Scenario | Consul | UI |', '|---|---|---|');
      for (const m of scenarios) {
        if (m) out.push(`| ${cell(m[1])} | ${cell(m[2])}${m[3] ? ` (${cell(m[3])})` : ''} | ${cell(m[4])} |`);
      }
      out.push('');
    }

    const byFile = new Map<string, TestCase[]>();
    for (const t of tests) {
      const key = path.relative(process.cwd(), t.location.file);
      byFile.set(key, [...(byFile.get(key) ?? []), t]);
    }
    out.push('| Spec file | Passed | Failed | Flaky | Skipped | Time |', '|---|---:|---:|---:|---:|---:|');
    for (const [specFile, specTests] of [...byFile].sort(([a], [b]) => a.localeCompare(b))) {
      const n = (outcome: ReturnType<TestCase['outcome']>) => specTests.filter((t) => t.outcome() === outcome).length;
      const ms = specTests.flatMap((t) => t.results).reduce((sum, r) => sum + r.duration, 0);
      out.push(
        `| ${n('unexpected') ? '❌' : '✅'} ${cell(specFile)} | ${n('expected')} | ${n('unexpected')} | ${n('flaky')} | ${n('skipped')} | ${seconds(ms)} |`,
      );
    }
    out.push('');

    for (const t of tests.filter((test) => test.outcome() === 'unexpected' || test.outcome() === 'flaky')) {
      const last = t.results[t.results.length - 1];
      const errors = t.outcome() === 'flaky' ? t.results.flatMap((r) => r.errors) : (last?.errors ?? []);
      const where = `${path.relative(process.cwd(), t.location.file)}:${t.location.line}`;
      out.push(
        `<details><summary>${t.outcome() === 'flaky' ? '⚠️ Flaky' : '❌ Failed'}: ${html(title(t))} (${html(where)})</summary>`,
        '',
        '```',
        ...errors.slice(0, 2).map(errorText),
        '```',
        '',
        '</details>',
        '',
      );
    }

    const { GITHUB_SERVER_URL: server, GITHUB_REPOSITORY: repo, GITHUB_RUN_ID: run } = process.env;
    if (server && repo && run) {
      out.push(`Full HTML report, traces and agent logs: [this run's artifacts](${server}/${repo}/actions/runs/${run}#artifacts).`, '');
    }
    return `${out.join('\n')}\n`;
  }
}

function title(test: TestCase): string {
  // titlePath() is ['', project, file, ...describes, title].
  return test.titlePath().slice(3).join(' › ');
}

function errorText(error: TestError): string {
  const text = stripAnsi(error.message ?? error.value ?? 'Unknown error');
  const lines = text.split('\n').slice(0, 15);
  const at = error.location ? `\n    at ${path.relative(process.cwd(), error.location.file)}:${error.location.line}` : '';
  return lines.join('\n').replaceAll('```', "'''") + at;
}

function stripAnsi(text: string): string {
  return text.replace(/\u001b\[[0-9;]*m/g, '');
}

function cell(text: string): string {
  return text.replaceAll('|', '\\|').replaceAll('\n', ' ');
}

function html(text: string): string {
  return text.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
}

function seconds(ms: number): string {
  return `${(ms / 1000).toFixed(1)}s`;
}
