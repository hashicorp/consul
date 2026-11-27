/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit acceptance-test harness.
//
// Shared setup and helpers for acceptance tests:
//
//   - setupApplicationTest + api-double lifecycle (startup / reset)
//   - the CONSUL_NSPACES_ENABLED "namespace matrix" (a scenario runs once per
//     namespace under Enterprise, once under CE) via `nspaceScenario`
//   - the @onlyNamespaceable / @notNamespaceable gating via options
//   - fixture setup, request-history and page-object assertions
//
// See tests/acceptance/dc/intentions/create-test.js for a worked example.

import { test, skip } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { settled, isSettled, getContext, click, fillIn, waitUntil } from '@ember/test-helpers';
import { findOne } from 'ember-cli-page-object/extend';

import { env } from 'consul-ui/env';
import api from 'consul-ui/tests/helpers/api';
import pages from 'consul-ui/tests/pages';

export { api, pages };
export { nspaceJSON, nspaceYAML, nspaceEndpoint } from 'consul-ui/tests/helpers/nspace-template';

// Captured once so afterEach can restore the <html> class list (prevents
// mutated feature-detection classes from leaking between tests).
const staticClassList = [...document.documentElement.classList];

const startup = function () {
  api.server.setCookie('CONSUL_LATENCY', 0);
};

// Abort all in-flight connections (blocking queries).
const purgeConnections = function (owner) {
  try {
    const connections = owner.lookup('service:client/connections');
    if (connections) {
      connections.purge();
    }
  } catch (e) {
    // service may already be destroyed
  }
};

const reset = function (owner) {
  // Wipe ALL cookies so mutated permission cookies don't leak between tests.
  document.cookie.split(';').forEach((c) => {
    const name = c.split('=')[0].trim();
    if (name) {
      document.cookie = `${name}=; expires=${new Date(0).toUTCString()}`;
    }
  });
  window.localStorage.clear();
  api.server.reset();

  purgeConnections(owner);

  const list = document.documentElement.classList;
  while (list.length > 0) {
    list.remove(list.item(0));
  }
  staticClassList.forEach((item) => list.add(item));
};

/**
 * Drop-in replacement for `setupApplicationTest(hooks)` that also wires up the
 * api-double startup/reset lifecycle. Call this once per `module`.
 */
export function setupAcceptanceTest(hooks) {
  setupApplicationTest(hooks);
  hooks.beforeEach(function () {
    startup();
  });
  hooks.afterEach(async function () {
    reset(this.owner);
    // A blocking query that was pausing between polls when we purged isn't a
    // connection yet; it starts afterwards and never returns, so `settled()`
    // would hang. Keep purging until nothing is pending.
    const owner = this.owner;
    try {
      await waitUntil(
        () => {
          purgeConnections(owner);
          return isSettled();
        },
        { timeout: 5000 }
      );
    } catch (e) {
      // fall through to settled(), which reports what is still pending
    }
    await settled();
  });
}

// The set of namespaces every namespaceable scenario is exercised against under
// Enterprise.
const NAMESPACE_MATRIX = ['', 'default', 'team-1', undefined];

const nspaceLabel = function (nspace) {
  if (nspace === '') return 'empty';
  if (typeof nspace === 'undefined') return 'undefined';
  return nspace;
};

/**
 * Define a scenario as one or more QUnit tests, reproducing the namespace
 * matrix and CE/ENT gating.
 *
 * - Enterprise (CONSUL_NSPACES_ENABLED):
 *     runs once per NAMESPACE_MATRIX entry, unless `notNamespaceable`.
 * - CE:
 *     runs once with nspace `undefined`, unless `onlyNamespaceable`.
 *
 * The callback receives `(assert, nspace)`; pass `nspace` on to `visit` so the
 * URL is prefixed correctly.
 *
 * @param {string} title
 * @param {(assert: object, nspace: string|undefined) => (void|Promise)} callback
 * @param {{ onlyNamespaceable?: boolean, notNamespaceable?: boolean, ignore?: boolean }} [options]
 */
export function nspaceScenario(title, callback, options = {}) {
  const { onlyNamespaceable = false, notNamespaceable = false, ignore = false } = options;

  if (ignore) {
    skip(title, function () {});
    return;
  }

  if (env('CONSUL_NSPACES_ENABLED')) {
    if (notNamespaceable) {
      return;
    }
    NAMESPACE_MATRIX.forEach((nspace) => {
      test(`${title} with the ${nspaceLabel(nspace)} namespace set`, function (assert) {
        return callback.call(this, assert, nspace);
      });
    });
  } else {
    if (onlyNamespaceable) {
      return;
    }
    test(title, function (assert) {
      return callback.call(this, assert, undefined);
    });
  }
}

// ---------------------------------------------------------------------------
// Navigation
// ---------------------------------------------------------------------------

let currentPage;

/**
 * Visit a registered page object (see tests/pages.js), applying the namespace
 * prefix for the given namespace. Returns the page object so
 * you can chain `.submit()` etc.
 *
 * This clears the api-double request history
 * first, so request assertions only see requests made since the last visit.
 * Calling `pages.X.visit()` or `@ember/test-helpers`' `visit` directly does not.
 *
 * `method` picks another visitable on the page object (e.g. `visitExported`).
 */
export function visit(name, params = {}, { nspace, method = 'visit' } = {}) {
  const page = pages[name];
  if (!page) {
    throw new Error(`Unknown page object: "${name}" (see tests/pages.js)`);
  }
  currentPage = page;
  api.server.clearHistory();
  const data = { ...params };
  if (nspace !== '' && typeof nspace !== 'undefined') {
    data.nspace = `~${nspace}`;
  }
  return page[method](data);
}

/**
 * Prefix a URL with the active namespace (`/dc-1/kv` becomes `/~team-1/dc-1/kv`
 * when a namespace is set). Returns `url` unchanged under CE / no namespace.
 */
export function nspaceURL(nspace, url) {
  if (nspace !== '' && typeof nspace !== 'undefined') {
    return `/~${nspace}${url}`;
  }
  return url;
}

/**
 * Click the nearest ancestor breadcrumb, i.e. the "back" link (what the old
 * `[data-test-back] a` pointed at).
 */
export function clickParentBreadcrumb() {
  const links = [
    ...document.querySelectorAll(
      '[data-test-breadcrumb-item]:not([data-test-breadcrumb-current]) a'
    ),
  ];
  if (links.length === 0) {
    throw new Error('No ancestor breadcrumb link found');
  }
  return click(links[links.length - 1]);
}

/** The most recently visited page object (for `.submit()`, `.fillIn()`, ...). */
export function page() {
  return currentPage;
}

/** Submit the current page's form. */
export function submit() {
  return currentPage.submit();
}

/**
 * Read the current URL via the app's configured location service (Consul uses a
 * custom location service, so the bare @ember/test-helpers currentURL is not
 * always correct here).
 */
export function currentURL() {
  const { owner } = getContext();
  const locationType = owner.lookup('service:env').var('locationType');
  const location = owner.lookup(`location:${locationType}`);
  return location.getURLFrom();
}

// ---------------------------------------------------------------------------
// Request-history assertions
// ---------------------------------------------------------------------------

/** The last Nth request (optionally filtered by method), newest-first. */
export function lastNthRequest(n, method) {
  let requests = api.server.history.slice(0).reverse();
  if (method) {
    requests = requests.filter((item) => item.method === method);
  }
  if (n == null) {
    return requests;
  }
  return requests[n];
}

/**
 * Assert a request of `method` was made to `url`. When `expected` is provided,
 * its `body` / `headers` keys are deep-compared against the matching request
 * payload.
 */
export function requestMade(assert, method, url, expected = {}) {
  const requests = lastNthRequest(null, method);
  const request = requests.find((item) => item.method === method && item.url === url);
  assert.ok(request, `Expected a ${method} request to ${url}`);
  if (!request) {
    return;
  }
  const body = expected.body || {};
  // Only parse when checking body fields: some requests (e.g. KV values) aren't JSON.
  const parsed =
    Object.keys(body).length > 0 && request.requestBody ? JSON.parse(request.requestBody) : {};
  Object.keys(body).forEach((key) => {
    assert.deepEqual(
      parsed[key],
      body[key],
      `Expected request body ${key} to equal ${JSON.stringify(body[key])}, was ${JSON.stringify(
        parsed[key]
      )}`
    );
  });
  const headers = expected.headers || {};
  Object.keys(headers).forEach((key) => {
    assert.deepEqual(
      request.requestHeaders[key],
      headers[key],
      `Expected request header ${key} to equal ${JSON.stringify(headers[key])}`
    );
  });
}

/** Assert no request of `method` was made to `url`. */
export function requestNotMade(assert, method, url) {
  const requests = lastNthRequest(null, method);
  const made = requests.some((item) => item.method === method && item.url === url);
  assert.notOk(made, `Did not expect a ${method} request to ${url}`);
}

/**
 * The value copied to the clipboard, as recorded by the test clipboard stub
 * (read from localStorage).
 */
export function clipboard() {
  return window.localStorage.getItem('clipboard');
}

/** Assert that every URL in `urls` was requested with `method`. */
export function requestsIncluded(assert, method, urls) {
  const made = new Set(lastNthRequest(null, method).map((item) => item.url));
  const missing = urls.filter((url) => !made.has(url));
  assert.equal(missing.length, 0, `Expected requests "${missing.join(', ')}"`);
}

/** Assert a request of `method` to `url` was made with no body. */
export function requestMadeWithNoBody(assert, method, url) {
  const request = lastNthRequest(null, method).find((item) => item.url === url);
  assert.ok(request, `Expected a ${method} request to ${url}`);
  if (request) {
    assert.equal(
      request.requestBody,
      null,
      `Expected the request body to be null, was ${request.requestBody}`
    );
  }
}

/** Assert the body of a `method` request to `url` has none of `properties`. */
export function requestMadeWithoutProperties(assert, method, url, properties) {
  const request = lastNthRequest(null, method).find((item) => item.url === url);
  assert.ok(request, `Expected a ${method} request to ${url}`);
  if (request) {
    const body = JSON.parse(request.requestBody);
    properties.forEach((key) => {
      assert.equal(typeof body[key], 'undefined', `Expected payload to not have a ${key} property`);
    });
  }
}

// ---------------------------------------------------------------------------
// Fixtures / api-double setup
// ---------------------------------------------------------------------------

// The app reads test settings from document.cookie (app/utils/get-environment.js)
// and the api-double reads its own cookie jar, so set both.
const setCookie = function (key, value) {
  document.cookie = `${key}=${value}`;
  api.server.setCookie(key, value);
};

/**
 * Create `count` models of `model`, optionally from fixture data. Creating
 * datacenters with a value also makes the first one the local datacenter.
 *
 * `model` is the api-double type name (`dc`, `service`, `node`, `kv`, `token`,
 * ...). Aliases such as `datacenters` or `nodes` are not mapped.
 */
export function createModels(model, count, value) {
  if (model === 'dc' && typeof value !== 'undefined') {
    document.cookie = `CONSUL_DATACENTER_LOCAL=${Array.isArray(value) ? value[0] : value}`;
  }
  api.server.createList(model, count, value);
}

/** Make requests to `url` (query string ignored) respond with `data`: `{ status, body, headers }`. */
export function respondWith(url, data) {
  const response = { ...data };
  if (typeof response.body !== 'undefined' && typeof response.body !== 'string') {
    response.body = JSON.stringify(response.body);
  }
  api.server.respondWith(url.split('?')[0], response);
}

/** Write each UI setting to localStorage as JSON. */
export function setSettings(data) {
  Object.keys(data).forEach((key) => {
    window.localStorage[key] = JSON.stringify(data[key]);
  });
}

/** Set the UI config the app reads at startup. */
export function setUiConfig(data) {
  document.cookie = `CONSUL_UI_CONFIG=${JSON.stringify(data)}`;
}

/** Set the local datacenter. */
export function setLocalDatacenter(value) {
  document.cookie = `CONSUL_DATACENTER_LOCAL=${value}`;
}

/** Disable ACLs (read by the env service from document.cookie). */
export function disableACLs() {
  document.cookie = 'CONSUL_ACLS_ENABLE=0';
}

/** Enable SSO. */
export function enableSSO() {
  setCookie('CONSUL_SSO_ENABLE', 1);
}

/** Enable admin partitions. */
export function enablePartitions() {
  setCookie('CONSUL_PARTITIONS_ENABLE', 1);
}

/** Set the default ACL policy (`allow` or `deny`). */
export function setDefaultACLPolicy(policy) {
  setCookie('CONSUL_ACL_POLICY', policy);
}

/** Enable the metrics proxy with the given metrics provider. */
export function setMetricsProvider(value) {
  document.cookie = 'CONSUL_METRICS_PROXY_ENABLE=1';
  document.cookie = `CONSUL_METRICS_PROVIDER=${value}`;
}

/** Set ACL permissions per resource and action, e.g. `{ key: { read: false } }`. */
export function setPermissions(data) {
  Object.entries(data).forEach(([resource, actions]) => {
    Object.entries(actions).forEach(([action, value]) => {
      setCookie(`CONSUL_RESOURCE_${resource.toUpperCase()}_${action.toUpperCase()}`, value);
    });
  });
}

/** Set the mock API response latency, in milliseconds. */
export function setLatency(ms) {
  setCookie('CONSUL_LATENCY', ms);
}

/** Set the API path prefix. */
export function setApiPrefix(prefix) {
  setCookie('CONSUL_API_PREFIX', prefix);
}

/** Store a legacy (pre-AccessorID) ACL token as the current token. */
export function useLegacyToken() {
  window.localStorage['consul:token'] = JSON.stringify({
    Namespace: 'default',
    AccessorID: null,
    SecretID: 'id',
  });
}

// ---------------------------------------------------------------------------
// Page-object queries
// ---------------------------------------------------------------------------

const objectAtPath = function (path) {
  return function (obj) {
    path.forEach((prop) => {
      obj = obj || {};
      obj = isNaN(parseInt(prop)) ? obj[prop] : obj.objectAt(parseInt(prop));
    });
    return obj;
  };
};

/**
 * Resolve a dotted path (e.g. `options.1.button` or `intentionList.intentions`)
 * on a page object, defaulting to the current page. Throws if it doesn't exist.
 */
export function find(path, root = currentPage) {
  const parts = path.split('.');
  const last = parts.pop();
  let parent = objectAtPath(parts)(root) || root;
  if (typeof parent.objectAt === 'function') {
    parent = parent.objectAt(0);
  }
  let obj = parent[last];
  if (typeof obj === 'undefined') {
    throw new Error(`PageObject not found: The '${path}' object doesn't exist`);
  }
  if (typeof obj === 'function') {
    obj = obj.bind(parent);
  }
  return obj;
}

/** Assert how many visible items a page-object collection has. */
export function assertVisibleCount(assert, collection, expected, label = 'items') {
  const actual = collection.filter((item) => item.isVisible).length;
  assert.equal(actual, expected, `Expected ${expected} ${label}, saw ${actual}`);
}

/**
 * Wait until a page-object collection has `expected` visible items, then assert
 * it (for lists that live-update through blocking queries).
 */
export async function waitForVisibleCount(assert, collection, expected, label = 'items') {
  const visible = () => collection.filter((item) => item.isVisible).length;
  try {
    await waitUntil(() => visible() === expected, { timeout: 5000 });
  } catch (e) {
    // fall through so the assertion below reports the actual count
  }
  assertVisibleCount(assert, collection, expected, label);
}

/**
 * Assert `property` on each item of a page-object collection equals the
 * corresponding entry in `expected`. Numbers in `expected` are compared as
 * strings because they come from the DOM.
 */
export function assertProperties(assert, collection, property, expected, label = 'items') {
  assert.ok(collection.length > 0, `Expected to find ${label}`);
  assertItemProperties(assert, collection.toArray(), property, expected, label);
}

/**
 * Like `assertProperties`, but compares the items in the order they appear on
 * screen, top to bottom (used to check sort order).
 */
export function assertPropertiesVertically(
  assert,
  collection,
  property,
  expected,
  label = 'items'
) {
  assert.ok(collection.length > 0, `Expected to find ${label}`);
  const items = collection
    .toArray()
    .map((item) => ({ item, top: findOne(item).getBoundingClientRect().top }))
    .sort((a, b) => a.top - b.top)
    .map(({ item }) => item);
  assertItemProperties(assert, items, property, expected, label);
}

const assertItemProperties = function (assert, items, property, expected, label) {
  items.forEach((item, i) => {
    const value = item[property];
    const actual = typeof value === 'undefined' ? null : value;
    const want = typeof expected[i] === 'number' ? expected[i].toString() : expected[i];
    assert.deepEqual(
      actual,
      want,
      `Expected ${property} on ${label}[${i}] as ${JSON.stringify(want)}, was ${JSON.stringify(
        actual
      )}`
    );
  });
};

// ---------------------------------------------------------------------------
// Forms and external providers
// ---------------------------------------------------------------------------

/**
 * Fill in `<input name="prefix[key]">` form fields. Fields rendered as an HDS
 * code editor (`aria-label="prefix[key]"`) are filled with `fillInCodeEditor`.
 */
export async function fillInForm(prefix, data) {
  for (const key of Object.keys(data)) {
    const name = `${prefix}[${key}]`;
    if (document.querySelector(`.cm-editor [aria-label="${name}"]`)) {
      await fillInCodeEditor(name, data[key]);
    } else {
      await fillIn(`[name="${name}"]`, data[key]);
    }
  }
}

/**
 * Fill in fields by name: an HDS code editor labelled `key` is filled with
 * `fillInCodeEditor`, anything else as `[name="key"]`.
 */
export async function fillInFields(data) {
  for (const key of Object.keys(data)) {
    if (document.querySelector(`.cm-editor [aria-label="${key}"]`)) {
      await fillInCodeEditor(key, data[key]);
    } else {
      await fillIn(`[name="${key}"]`, data[key]);
    }
  }
}

/**
 * Replace the content of the HDS code editor labelled `name`, which CodeMirror
 * picks up as user input. Throws if the field is a read-only code block or
 * doesn't exist, so "can't fill in" checks can use `assert.rejects`.
 */
export async function fillInCodeEditor(name, value) {
  const element = document.querySelector(`[aria-label="${name}"]`);
  if (!element || !element.closest('.cm-editor')) {
    if (element && element.closest('.hds-code-block')) {
      throw new Error(`The ${name} editor is set to readonly`);
    }
    throw new Error(`No code editor labelled ${name}`);
  }
  const content = document.createElement('div');
  content.textContent = value;
  element.innerHTML = '';
  element.appendChild(content);
  // CodeMirror reads DOM changes from a MutationObserver, so let it run first.
  await new Promise((resolve) => setTimeout(resolve, 0));
  await settled();
}

/** Make the OIDC popup resolve with `response` instead of opening a window. */
export function stubOIDCProvider(response) {
  const { owner } = getContext();
  const provider = owner.lookup('torii-provider:oidc-with-url');
  provider.popup.open = async function () {
    return response;
  };
}
