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
 * Reduce a list of checks to the shape `Consul::HealthBadge` expects, where
 * `count` is how many checks are in the displayed (worst) status and `passing`
 * is how many are passing — the badge needs the latter to tell "all checks
 * failing" apart from "3/6 checks failing".
 */
export function healthFromChecks(checks = []) {
  const status = statusFromChecks(checks);
  return {
    status,
    count: checks.filter((c) => c.Status === status).length,
    total: checks.length,
    passing: checks.filter((c) => c.Status === 'passing').length,
  };
}

/**
 * Worst status across a list of service instances, used for the overall health
 * badge next to a service's title.
 *
 * This reads each instance's own `Status` rather than flattening their checks:
 * `Checks` is an Ember fragment array rather than a real Array, so concatenating
 * them silently yields a list of array objects instead of checks.
 */
export function statusFromInstances(instances = []) {
  const statuses = [];
  (instances || []).forEach((instance) => statuses.push(instance.Status));

  if (statuses.includes('critical')) return 'critical';
  if (statuses.includes('warning')) return 'warning';
  if (statuses.includes('passing')) return 'passing';
  return 'empty';
}
