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
import { settled, getContext } from '@ember/test-helpers';

import { env } from 'consul-ui/env';
import api from 'consul-ui/tests/helpers/api';
import pages from 'consul-ui/tests/pages';

export { api, pages };

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
