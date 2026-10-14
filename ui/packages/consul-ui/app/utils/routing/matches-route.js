/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

/**
 * Whether `current` is `route` or one of its child routes.
 * Like the `is-href` helper, `dc.` routes also match their legacy
 * `nspace.`-prefixed equivalents.
 */
export default function matchesRoute(current, route) {
  if (typeof current !== 'string' || typeof route !== 'string') {
    return false;
  }
  if (current.startsWith('nspace.') && route.startsWith('dc.')) {
    route = `nspace.${route}`;
  }
  return current === route || current.startsWith(`${route}.`);
}
