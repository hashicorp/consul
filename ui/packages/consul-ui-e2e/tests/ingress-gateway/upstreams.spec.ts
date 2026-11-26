/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import type { Page } from '@playwright/test';
import { test, expect } from '../../fixtures/consul-scenario.ts';

test.use({ scenario: 'ingress-gateway' });

/**
 * Scenario: ingress-gateway
 * Route: /ui/dc1/services/ingress-gateway/upstreams (dc.services.show.upstreams)
 *
 * That tab (`app/templates/dc/services/show/upstreams.hbs`) only renders when
 * `item.Service.Kind === 'ingress-gateway' && !item.Service.PeerName`
 * (`app/templates/dc/services/show.hbs`), and lists upstreams with
 * `Consul::Upstream::Table` (`app/components/consul/upstream/table/index.hbs`),
 * one `<tr data-test-tabular-row>` per upstream. Per row it renders:
 *   - the service name, linked (`<a data-test-service-name>`) only when
 *     `item.InstanceCount > 0`, else a plain `<span data-test-service-name>`
 *   - a health cell, `<td data-test-service-health={{item.MeshStatus}}>`
 *   - an address cell, `<td data-test-upstream-address>`, with one entry per
 *     `item.GatewayConfig.Addresses` value, each rendering
 *     `<service>.ingress.<dc>.consul:<port>`.
 *
 * See `scenarios/ingress-gateway/{README.md,seed.ts}` for exactly what's
 * seeded and why (including the InstanceCount-inflation quirk asserted
 * below).
 */
const SERVICE = '/ui/dc1/services/ingress-gateway';
const UPSTREAMS = `${SERVICE}/upstreams`;

// Upstream names are pairwise non-substrings of each other, so filtering by
// name text is enough to scope to a single row.
function row(page: Page, name: string) {
  return page.locator('[data-test-tabular-row]').filter({ hasText: name });
}

function addresses(r: ReturnType<typeof row>, name: string) {
  return r.locator('[data-test-upstream-address]').getByText(new RegExp(`${name}\\.ingress\\.dc1\\.consul:\\d+`));
}

test.describe('Ingress gateway — Upstreams tab', () => {
  test('is reachable from the service page and lists all 5 linked upstreams', async ({ page }) => {
    await page.goto(SERVICE);
    await page.locator('[data-test-tab="tab_upstreams"]').click();
    await expect(page).toHaveURL(/\/upstreams$/);

    await expect(page.locator('[data-test-service-name]')).toHaveCount(5);
    for (const name of ['upstream-01', 'upstream-02', 'upstream-03', 'upstream-04', 'upstream-05']) {
      await expect(row(page, name).locator('[data-test-service-name]')).toHaveText(name);
    }
  });

  test('upstream-01/02/03 render linked with their aggregated health status', async ({ page }) => {
    await page.goto(UPSTREAMS);

    // upstream-01: single passing instance -> aggregated MeshStatus "passing".
    // upstream-02: 2nd of 2 instances is critical -> aggregated "critical".
    // upstream-03: 3rd of 3 instances is warning -> aggregated "warning".
    for (const [name, status] of [
      ['upstream-01', 'passing'],
      ['upstream-02', 'critical'],
      ['upstream-03', 'warning'],
    ]) {
      const r = row(page, name);
      await expect(r.locator('a[data-test-service-name]')).toHaveAttribute('href', `/ui/dc1/services/${name}`);
      await expect(r.locator('[data-test-service-health]')).toHaveAttribute('data-test-service-health', status);
    }
  });

  test('upstream-04 (linked from 2 listeners) shows two addresses and stays linked', async ({ page, consul }) => {
    // Backend cross-check for the documented InstanceCount quirk: 1 real
    // catalog instance, but linked from 2 listeners -> reported
    // InstanceCount 2 (agent/consul/internal_endpoint.go counts once per
    // GatewayService mapping, i.e. once per listener, with no
    // de-duplication) — see scenarios/ingress-gateway/README.md.
    const rows = await consul.gatewayServicesNodes('ingress-gateway');
    const upstream04Row = rows.find((r) => r.Name === 'upstream-04');
    expect(upstream04Row?.InstanceCount).toBe(2);

    await page.goto(UPSTREAMS);
    const upstream04 = row(page, 'upstream-04');
    // A real instance exists, so the name is still linked with a health status.
    await expect(upstream04.locator('a[data-test-service-name]')).toHaveAttribute('href', '/ui/dc1/services/upstream-04');
    await expect(upstream04.locator('[data-test-service-health]')).toHaveAttribute('data-test-service-health', 'passing');

    // One address per listener it's linked from (ports 8084 and 8090).
    await expect(addresses(upstream04, 'upstream-04')).toHaveText([
      'upstream-04.ingress.dc1.consul:8084',
      'upstream-04.ingress.dc1.consul:8090',
    ]);
  });

  test('upstream-05 (never registered) renders unlinked with no health status, but keeps its address', async ({ page }) => {
    await page.goto(UPSTREAMS);
    const upstream05 = row(page, 'upstream-05');

    // Plain unlinked name -- InstanceCount is 0.
    await expect(upstream05.locator('span[data-test-service-name]')).toHaveText('upstream-05');
    await expect(upstream05.locator('a[data-test-service-name]')).toHaveCount(0);

    // No aggregated health status for an unregistered upstream.
    await expect(upstream05.locator('[data-test-service-health]')).not.toHaveAttribute(
      'data-test-service-health',
      /^(passing|warning|critical)$/,
    );

    // The listener/port still produces an address, regardless of whether the
    // upstream is actually registered in the catalog.
    await expect(addresses(upstream05, 'upstream-05')).toHaveText(['upstream-05.ingress.dc1.consul:8085']);
  });

  test('every row\'s address matches <service>.ingress.dc1.consul:<port>', async ({ page }) => {
    await page.goto(UPSTREAMS);

    const expected: Record<string, string[]> = {
      'upstream-01': ['upstream-01.ingress.dc1.consul:8081'],
      'upstream-02': ['upstream-02.ingress.dc1.consul:8082'],
      'upstream-03': ['upstream-03.ingress.dc1.consul:8083'],
      'upstream-04': ['upstream-04.ingress.dc1.consul:8084', 'upstream-04.ingress.dc1.consul:8090'],
      'upstream-05': ['upstream-05.ingress.dc1.consul:8085'],
    };

    for (const [name, values] of Object.entries(expected)) {
      await expect(addresses(row(page, name), name)).toHaveText(values);
    }
  });
});
