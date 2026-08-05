/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import { assert } from '@ember/debug';
import { routes } from 'consul-ui/router';

/**
 * @typedef {Object} BreadcrumbItem
 * @property {string}  label       - Human-readable label for the crumb.
 * @property {string}  route       - Fully-qualified Ember route name.
 * @property {Object}  params      - Route dynamic-segment params (subset of routeParams).
 * @property {boolean} isCurrent   - True only for the last item in the chain.
 * @property {boolean} isClickable - False only for the current (last) item.
 */

/**
 * BreadcrumbsService
 *
 * Walks the `parent` chain encoded in each route's `breadcrumb` metadata
 * block (stored in `vendor/consul-ui/routes.js`) and returns an ordered
 * `BreadcrumbItem[]` for the given route name and its current URL params.
 *
 * The traversal is purely entity-level: it follows the declared `parent`
 * links rather than the browser's navigation history, so the breadcrumb
 * trail is always deterministic regardless of how the user arrived at a page.
 */
export default class BreadcrumbsService extends Service {
  // ─── Public API ───────────────────────────────────────────────────────────

  /**
   * Returns `true` when the given route should display breadcrumbs.
   * A route opts out by setting `breadcrumb: { show: false }`.
   *
   * @param {string} routeName
   * @returns {boolean}
   */
  shouldShowBreadcrumbs(routeName) {
    const config = this.getBreadcrumbConfig(routeName);
    if (!config) return false;
    return config.show !== false;
  }

  /**
   * Returns the raw `breadcrumb` config block for the given route, or
   * `null` if the route does not exist / has no breadcrumb metadata.
   *
   * @param {string} routeName - Dot-separated Ember route name, e.g. `dc.services.show`.
   * @returns {Object|null}
   */
  getBreadcrumbConfig(routeName) {
    // Manually traverse the nested plain-object tree so that a missing
    // intermediate key returns null cleanly (Ember's `get()` is designed for
    // observable objects and can produce unexpected results on plain JSON).
    const parts = routeName.split('.');
    let node = this._routeTree();
    for (const part of parts) {
      if (node == null || typeof node !== 'object') return null;
      node = node[part];
    }
    return node?._options?.breadcrumb ?? null;
  }

  /**
   * Computes the full breadcrumb chain for `routeName` using `routeParams`
   * for dynamic label resolution.
   *
   * The chain is built by walking the `parent` references starting from
   * `routeName` back to the root, then reversing so it reads root → leaf.
   * The last item always gets `isCurrent: true, isClickable: false`.
   *
   * @param {string} routeName   - Current fully-qualified route name.
   * @param {Object} routeParams - All available URL params (dc, name, id, …).
   * @returns {BreadcrumbItem[]}
   */
  computeBreadcrumbs(routeName, routeParams = {}) {
    const items = [];
    let current = routeName;
    // Guard against cycles in the parent chain (e.g. A.parent=B, B.parent=A).
    const visited = new Set();

    // Walk up the parent chain until there is no parent reference.
    while (current) {
      if (visited.has(current)) break;
      visited.add(current);

      const config = this.getBreadcrumbConfig(current);
      if (!config) break;

      items.unshift({
        label: this._resolveLabel(config.label, routeParams),
        route: current,
        // `models` is the ordered array of dynamic-segment values for this
        // route extracted from path segments (e.g. ['dc-1'] for dc.nodes,
        // ['dc-1', 'web'] for dc.services.show).  Used by the template to
        // pass explicit models to href-to, avoiding the fsm-with-optional
        // location inheriting extra optional segments (partition/nspace/peer).
        models: this._modelsFor(current, routeParams),
        params: routeParams,
        isCurrent: false,
        isClickable: true,
      });

      current = config.parent ?? null;
    }

    // Mark the leaf as current / non-clickable.
    if (items.length > 0) {
      const last = items[items.length - 1];
      last.isCurrent = true;
      last.isClickable = false;
    }

    return items;
  }

  // ─── Private helpers ──────────────────────────────────────────────────────

  /**
   * Returns the ordered array of dynamic-segment values for `routeName` by
   * walking the route tree and extracting `:param` tokens from each node's
   * `path` option in order from root → leaf.
   *
   * Example: `_modelsFor('dc.services.show', { dc: 'dc-1', name: 'web' })`
   *   → `['dc-1', 'web']`
   *
   * Uses `_routeTree()` (overridable in tests) so the same fixture that
   * patches `getBreadcrumbConfig` also covers `_modelsFor` automatically.
   *
   * @param {string} routeName
   * @param {Object} routeParams
   * @returns {Array}
   */
  _modelsFor(routeName, routeParams) {
    const parts = routeName.split('.');
    let node = this._routeTree();
    const models = [];

    for (const part of parts) {
      if (node == null || typeof node !== 'object') break;
      node = node[part];
      const path = node?._options?.path ?? '';
      // Extract all `:paramName` tokens from the path segment.
      for (const match of path.matchAll(/:([^/]+)/g)) {
        const key = match[1];
        if (Object.prototype.hasOwnProperty.call(routeParams, key)) {
          models.push(routeParams[key]);
        }
      }
    }

    return models;
  }

  /**
   * Returns the root route tree.  Extracted so tests can override it by
   * setting `service._testRoutes` — the same mechanism used for
   * `getBreadcrumbConfig`.
   *
   * @returns {Object}
   */
  _routeTree() {
    return this._testRoutes ?? routes;
  }

  /**
   * If `label` matches a key in `routeParams` return its value;
   * otherwise return `label` as a static string.
   *
   * @param {string} label
   * @param {Object} routeParams
   * @returns {string}
   */
  _resolveLabel(label, routeParams) {
    if (label && Object.prototype.hasOwnProperty.call(routeParams, label)) {
      return routeParams[label];
    }
    assert('[BreadcrumbsService] route breadcrumb is missing a `label` key', label != null);
    return label ?? '';
  }
}
