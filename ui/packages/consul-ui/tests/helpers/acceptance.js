/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Native QUnit acceptance-test harness.
//
// This is the scaffolding that lets a `.feature` file be rewritten as a plain
// QUnit acceptance test while preserving the behaviour that the yadda runner
// (tests/helpers/yadda-annotations.js) gives us for free:
//
//   - setupApplicationTest + api-double lifecycle (startup / reset)
//   - the CONSUL_NSPACES_ENABLED "namespace matrix" (a scenario runs once per
//     namespace under Enterprise, once under CE) via `nspaceScenario`
//   - the @onlyNamespaceable / @notNamespaceable gating via options
//   - request-history assertions equivalent to tests/steps/assertions/http.js
//
// Migrating a feature: see tests/acceptance/dc/intentions/create-test.js for a
// worked example, then delete the corresponding `.feature` and `-steps.js`.

import { test, skip } from 'qunit';
import { setupApplicationTest } from 'ember-qunit';
import { settled, getContext, click } from '@ember/test-helpers';

import { env } from 'consul-ui/env';
import api from 'consul-ui/tests/helpers/api';
import pages from 'consul-ui/tests/pages';

export { api, pages };
export { nspaceJSON, nspaceYAML, nspaceEndpoint } from 'consul-ui/tests/helpers/nspace-template';

// Captured once so afterEach can restore the <html> class list exactly like the
// yadda reset() does (prevents mutated feature-detection classes from leaking).
const staticClassList = [...document.documentElement.classList];

const startup = function () {
  api.server.setCookie('CONSUL_LATENCY', 0);
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

  // Abort all in-flight connections (blocking queries).
  try {
    const connections = owner.lookup('service:client/connections');
    if (connections) {
      connections.purge();
    }
  } catch (e) {
    // service may already be destroyed
  }

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
    await settled();
  });
}

// The set of namespaces every namespaceable scenario is exercised against under
// Enterprise. Mirrors tests/helpers/yadda-annotations.js.
const NAMESPACE_MATRIX = ['', 'default', 'team-1', undefined];

const nspaceLabel = function (nspace) {
  if (nspace === '') return 'empty';
  if (typeof nspace === 'undefined') return 'undefined';
  return nspace;
};

/**
 * Define a scenario as one or more QUnit tests, reproducing the namespace
 * matrix and CE/ENT gating that yadda applied via annotations.
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
 * prefix the same way the yadda `visit` step does. Returns the page object so
 * you can chain `.submit()` etc.
 */
export function visit(name, params = {}, { nspace } = {}) {
  const page = pages[name];
  if (!page) {
    throw new Error(`Unknown page object: "${name}" (see tests/pages.js)`);
  }
  currentPage = page;
  const data = { ...params };
  if (nspace !== '' && typeof nspace !== 'undefined') {
    data.nspace = `~${nspace}`;
  }
  return page.visit(data);
}

/**
 * Prefix a URL with the active namespace, matching the yadda url dictionary
 * (which rewrites `the url should be /dc-1/kv` to `/~team-1/dc-1/kv` when a
 * namespace is set). Returns `url` unchanged under CE / no namespace.
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

/** Submit the current page's form (equivalent to the `I submit` step). */
export function submit() {
  return currentPage.submit();
}

/**
 * Read the current URL via the app's configured location service, matching the
 * yadda `the url should be` step (Consul uses a custom location service, so the
 * bare @ember/test-helpers currentURL is not always correct here).
 */
export function currentURL() {
  const { owner } = getContext();
  const locationType = owner.lookup('service:env').var('locationType');
  const location = owner.lookup(`location:${locationType}`);
  return location.getURLFrom();
}

// ---------------------------------------------------------------------------
// Request-history assertions (equivalent to tests/steps/assertions/http.js)
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
 * payload (equivalent to `a X request was made to "..." from yaml`).
 */
export function requestMade(assert, method, url, expected = {}) {
  const requests = lastNthRequest(null, method);
  const request = requests.find((item) => item.method === method && item.url === url);
  assert.ok(request, `Expected a ${method} request to ${url}`);
  if (!request) {
    return;
  }
  const body = expected.body || {};
  const parsed = request.requestBody ? JSON.parse(request.requestBody) : {};
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
 * (equivalent to the yadda `I copied "..."` step, which reads localStorage).
 */
export function clipboard() {
  return window.localStorage.getItem('clipboard');
}

/**
 * Assert that every URL in `urls` was requested with `method`
 * (equivalent to `the last $method requests included from yaml`).
 */
export function requestsIncluded(assert, method, urls) {
  const made = new Set(lastNthRequest(null, method).map((item) => item.url));
  const missing = urls.filter((url) => !made.has(url));
  assert.equal(missing.length, 0, `Expected requests "${missing.join(', ')}"`);
}

/**
 * Assert a request of `method` to `url` was made with no body (equivalent to
 * `a $method request was made to "$endpoint" with no body`).
 */
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

/**
 * Assert the body of a `method` request to `url` has none of `properties`
 * (equivalent to `... was made to "$endpoint" without properties from yaml`).
 */
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
// Fixtures / api-double setup (the `Given ...` steps)
// ---------------------------------------------------------------------------

const setCookie = function (key, value) {
  document.cookie = `${key}=${value}`;
  api.server.setCookie(key, value);
};

/**
 * Create `count` models of `model`, optionally from fixture data
 * (`Given $number $model models [with the value "..."|from yaml]`). Creating
 * datacenters with a value also makes the first one the local datacenter, as
 * the yadda step did.
 */
export function createModels(model, count, value) {
  if (model === 'dc' && typeof value !== 'undefined') {
    document.cookie = `CONSUL_DATACENTER_LOCAL=${Array.isArray(value) ? value[0] : value}`;
  }
  api.server.createList(model, count, value);
}

/** `Given the url "$endpoint" responds with ...` - `data` is `{ status, body, headers }`. */
export function respondWith(url, data) {
  const response = { ...data };
  if (typeof response.body !== 'undefined' && typeof response.body !== 'string') {
    response.body = JSON.stringify(response.body);
  }
  api.server.respondWith(url.split('?')[0], response);
}

/** `Given settings from yaml` - writes each key to localStorage as JSON. */
export function setSettings(data) {
  Object.keys(data).forEach((key) => {
    window.localStorage[key] = JSON.stringify(data[key]);
  });
}

/** `Given ui_config from yaml` */
export function setUiConfig(data) {
  document.cookie = `CONSUL_UI_CONFIG=${JSON.stringify(data)}`;
}

/** `Given the local datacenter is "$value"` */
export function setLocalDatacenter(value) {
  document.cookie = `CONSUL_DATACENTER_LOCAL=${value}`;
}

/** `Given ACLs are disabled` (read by the env service from document.cookie). */
export function disableACLs() {
  document.cookie = 'CONSUL_ACLS_ENABLE=0';
}

/** `Given SSO is enabled` */
export function enableSSO() {
  setCookie('CONSUL_SSO_ENABLE', 1);
}

/** `Given partitions are enabled` */
export function enablePartitions() {
  setCookie('CONSUL_PARTITIONS_ENABLE', 1);
}

/** `Given the default ACL policy is "$policy"` */
export function setDefaultACLPolicy(policy) {
  api.server.setCookie('CONSUL_ACL_POLICY', policy);
}

/** `Given a "$value" metrics provider` */
export function setMetricsProvider(value) {
  document.cookie = 'CONSUL_METRICS_PROXY_ENABLE=1';
  document.cookie = `CONSUL_METRICS_PROVIDER=${value}`;
}

/** `Given permissions from yaml`, e.g. `{ key: { read: false } }`. */
export function setPermissions(data) {
  Object.entries(data).forEach(([resource, actions]) => {
    Object.entries(actions).forEach(([action, value]) => {
      api.server.setCookie(
        `CONSUL_RESOURCE_${resource.toUpperCase()}_${action.toUpperCase()}`,
        value
      );
    });
  });
}

/** `Given a network latency of $number` */
export function setLatency(ms) {
  api.server.setCookie('CONSUL_LATENCY', ms);
}

/** `Given an API prefix of "$prefix"` */
export function setApiPrefix(prefix) {
  api.server.setCookie('CONSUL_API_PREFIX', prefix);
}

/** `Given I'm using a legacy token` */
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

/**
 * Assert how many visible items a page-object collection has (equivalent to
 * `I see $num $model models [on the $component component]`).
 */
export function assertVisibleCount(assert, collection, expected, label = 'items') {
  const actual = collection.filter((item) => item.isVisible).length;
  assert.equal(actual, expected, `Expected ${expected} ${label}, saw ${actual}`);
}

/**
 * Assert `property` on each item of a page-object collection equals the
 * corresponding entry in `expected` (equivalent to
 * `I see $property on the $component like yaml`). Numbers in `expected` are
 * compared as strings because they come from the DOM.
 */
export function assertProperties(assert, collection, property, expected, label = 'items') {
  assert.ok(collection.length > 0, `Expected to find ${label}`);
  for (let i = 0; i < collection.length; i++) {
    const value = collection.objectAt(i)[property];
    const actual = typeof value === 'undefined' ? null : value;
    const want = typeof expected[i] === 'number' ? expected[i].toString() : expected[i];
    assert.deepEqual(
      actual,
      want,
      `Expected ${property} on ${label}[${i}] as ${JSON.stringify(want)}, was ${JSON.stringify(
        actual
      )}`
    );
  }
}
