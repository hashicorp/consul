/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/**
 * Shared health-status helpers.
 *
 * The service-instance table derives a row's health from a list of individual
 * checks, while the services / upstreams tables get pre-aggregated counts from
 * the API (`MeshStatus`, `MeshChecksPassing`, ...). Both end up feeding the
 * same `Consul::HealthBadge`, so the precedence rules live here.
 */

// Coarse health ordering used by the health column sort comparators
// (critical first, then warning, passing, empty/unknown last).
export const STATUS_ORDER = { critical: 0, warning: 1, passing: 2, empty: 3 };

/**
 * Worst status present in a list of checks. A single critical check makes the
 * whole group critical, and so on down the precedence chain.
 */
export function statusFromChecks(checks = []) {
  if (checks.some((c) => c.Status === 'critical')) return 'critical';
  if (checks.some((c) => c.Status === 'warning')) return 'warning';
  if (checks.some((c) => c.Status === 'passing')) return 'passing';
  return 'empty';
}

/**
 * Reduce a list of checks to the `{ status, count, total }` shape
 * `Consul::HealthBadge` expects, where `count` is how many checks are in the
 * displayed (worst) status.
 */
export function healthFromChecks(checks = []) {
  const status = statusFromChecks(checks);
  return {
    status,
    count: checks.filter((c) => c.Status === status).length,
    total: checks.length,
  };
}
