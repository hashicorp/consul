/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

function shallowEqual(a = {}, b = {}) {
  const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
  return [...keys].every((key) => a[key] === b[key]);
}

/**
 * Whether two RouteInfo objects describe the same route tree with identical
 * params and query params. This is the check ember-a11y-refocus's default
 * route change validator uses to decide that nothing changed.
 */
export default function routeInfoEqual(a, b) {
  do {
    if (!a || !b) {
      return false;
    }
    if (
      a.name !== b.name ||
      !shallowEqual(a.params, b.params) ||
      !shallowEqual(a.queryParams, b.queryParams)
    ) {
      return false;
    }
    a = a.parent;
    b = b.parent;
  } while (a || b);
  return true;
}
