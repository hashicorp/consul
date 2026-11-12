/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Namespace placeholder substitution for native acceptance tests.
//
// Acceptance fixtures embed `@namespace` / `@!namespace` placeholders so the
// same scenario can run against every namespace. These helpers apply the same
// rewrites the (now deprecated) tests/dictionary.js applied to its `json`,
// `yaml` and `endpoint` terms:
//
//   @namespace    the active namespace, or `default` when none is set. In URLs
//                 and YAML the surrounding `ns=` / `Namespace:` is dropped
//                 entirely when no namespace is set.
//   @!namespace   "always": the namespace (or `default`) under Enterprise, and
//                 removed under CE.

import YAML from 'js-yaml';
import { env } from 'consul-ui/env';

const isSet = (nspace) => typeof nspace !== 'undefined' && nspace !== '';
const nspaceOrDefault = (nspace) => (isSet(nspace) ? nspace : 'default');
const enabled = () => !!env('CONSUL_NSPACES_ENABLED');

/**
 * Replace `@namespace` in a JSON string and parse it.
 */
export function nspaceJSON(str, nspace) {
  return JSON.parse(str.replace(/@namespace/g, nspaceOrDefault(nspace)));
}

/**
 * Apply the namespace rewrites to a YAML string and parse it.
 *
 * @param {string} str
 * @param {string|undefined} nspace
 * @param {{ nspacesEnabled?: boolean }} [options] defaults to CONSUL_NSPACES_ENABLED
 */
export function nspaceYAML(str, nspace, { nspacesEnabled = enabled() } = {}) {
  let val = str;
  if (nspacesEnabled) {
    val = val.replace(/ns=@!namespace/g, `ns=${nspaceOrDefault(nspace)}`);
    val = val.replace(/Namespace: @!namespace/g, `Namespace: ${nspaceOrDefault(nspace)}`);
  } else {
    val = val.replace(/&ns=@!namespace/g, '');
    val = val.replace(/&ns=\*/g, '');
    val = val.replace(/- \/v1\/namespaces/g, '');
    val = val.replace(/Namespace: @!namespace/g, '');
  }
  if (!isSet(nspace)) {
    val = val.replace(/Namespace: @namespace/g, '').replace(/&ns=@namespace/g, '');
  }
  val = val.replace(/@namespace/g, nspaceOrDefault(nspace));
  return YAML.load(val);
}

/**
 * Apply the namespace rewrites to an endpoint (URL) string.
 *
 * @param {string} str
 * @param {string|undefined} nspace
 * @param {{ nspacesEnabled?: boolean }} [options] defaults to CONSUL_NSPACES_ENABLED
 */
export function nspaceEndpoint(str, nspace, { nspacesEnabled = enabled() } = {}) {
  let val = str;
  if (nspacesEnabled) {
    val = val.replace(/ns=@!namespace/g, `ns=${nspaceOrDefault(nspace)}`);
  } else {
    val = val.replace(/&ns=@!namespace/g, '');
    val = val.replace(/&ns=\*/g, '');
  }
  if (isSet(nspace)) {
    val = val.replace(/ns=@namespace/g, `ns=${nspace}`);
  } else {
    val = val
      .replace(/&ns=@namespace/g, '')
      .replace(/ns=@namespace&/g, '')
      .replace(/ns=@namespace/g, '');
  }
  return val;
}
