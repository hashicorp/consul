/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import getEnvironment from 'consul-ui/utils/get-environment';
import { module, test } from 'qunit';
const getEntriesByType = function (type) {
  return [
    {
      initiatorType: 'script',
      name: '',
      nextHopProtocol: 'spdy',
    },
  ];
};
const makeGetElementsBy = function (str) {
  return function (name) {
    return [
      {
        src: str,
        content: str,
      },
    ];
  };
};
const makeOperatorConfig = function (json) {
  return {
    textContent: JSON.stringify(json),
  };
};
const win = {
  performance: {
    getEntriesByType: getEntriesByType,
  },
  location: {
    hash: '',
  },
  localStorage: {
    getItem: function (key) {},
  },
};
const doc = {
  cookie: '',
  getElementsByTagName: makeGetElementsBy(''),
  getElementsByName: makeGetElementsBy('{}'),
  querySelector: () => makeOperatorConfig({}),
};
module('Unit | Utility | getEnvironment', function () {
  test('it returns a function', function (assert) {
    const config = {};
    const env = getEnvironment(config, win, doc);
    assert.strictEqual(typeof env, 'function');
  });
  test('it returns the correct operator value', function (assert) {
    const config = {};
    const env = getEnvironment(config, win, doc);
    assert.equal(env('CONSUL_HTTP_PROTOCOL'), 'spdy');
  });
  test('it returns the correct operator value when set via config', function (assert) {
    const config = {
      CONSUL_HTTP_PROTOCOL: 'hq',
    };
    const env = getEnvironment(config, win, doc);
    assert.equal(env('CONSUL_HTTP_PROTOCOL'), 'hq');
  });
  test('it returns the correct URL for the root of the UI', function (assert) {
    let config = {
      environment: 'production',
    };
    let expected = 'http://localhost/ui';
    let doc = {
      cookie: '',
      getElementsByTagName: makeGetElementsBy(`${expected}/assets/consul-ui.js`),
      getElementsByName: makeGetElementsBy('{}'),
      querySelector: () => makeOperatorConfig({}),
    };
    let env = getEnvironment(config, win, doc);
    assert.equal(env('CONSUL_BASE_UI_URL'), expected);
    expected = 'http://localhost/somewhere/else';
    doc = {
      cookie: '',
      getElementsByTagName: makeGetElementsBy(`${expected}/assets/consul-ui.js`),
      getElementsByName: makeGetElementsBy('{}'),
      querySelector: () => makeOperatorConfig({}),
    };
    env = getEnvironment(config, win, doc);
    assert.equal(env('CONSUL_BASE_UI_URL'), expected);
  });

  test('it returns the correct max connections depending on protocol', function (assert) {
    let config = {
      CONSUL_HTTP_PROTOCOL: 'hq',
    };
    let env = getEnvironment(config, win, doc);
    assert.equal(env('CONSUL_HTTP_MAX_CONNECTIONS'), undefined);
    config = {
      CONSUL_HTTP_PROTOCOL: 'http/1.1',
    };
    env = getEnvironment(config, win, doc);
    assert.equal(env('CONSUL_HTTP_MAX_CONNECTIONS'), 5);
  });
  test('it returns the correct max connections if performance.getEntriesByType is not available', function (assert) {
    const config = {};
    let win = {};
    let env = getEnvironment(config, win, doc);
    assert.equal(env('CONSUL_HTTP_MAX_CONNECTIONS'), 5);
    win = {
      performance: {},
    };
    env = getEnvironment(config, win, doc);
    assert.equal(env('CONSUL_HTTP_MAX_CONNECTIONS'), 5);
  });
  test('it returns the correct user value', function (assert) {
    const config = {};
    let win = {
      localStorage: {
        getItem: function (key) {
          return '1';
        },
      },
    };
    let env = getEnvironment(config, win, doc);
    assert.ok(env('CONSUL_UI_DISABLE_REALTIME'));
    win = {
      localStorage: {
        getItem: function (key) {
          return '0';
        },
      },
    };
    env = getEnvironment(config, win, doc);
    assert.notOk(env('CONSUL_UI_DISABLE_REALTIME'));
    win = {
      localStorage: {
        getItem: function (key) {
          return null;
        },
      },
    };
    env = getEnvironment(config, win, doc);
    assert.notOk(env('CONSUL_UI_DISABLE_REALTIME'));
  });
  test('it returns the correct user value when set via config', function (assert) {
    const config = {
      CONSUL_UI_DISABLE_REALTIME: true,
    };
    const env = getEnvironment(config, win, doc);
    assert.ok(env('CONSUL_UI_DISABLE_REALTIME'));
  });
  test('it returns the correct dev value (via cookies)', function (assert) {
    let config = {
      environment: 'test',
      CONSUL_NSPACES_ENABLED: false,
    };
    let doc = {
      cookie: 'CONSUL_NSPACES_ENABLE=1',
      getElementsByTagName: makeGetElementsBy(''),
      getElementsByName: makeGetElementsBy('{}'),
      querySelector: () => makeOperatorConfig({ NamespacesEnabled: true }),
    };
    let env = getEnvironment(config, win, doc);
    assert.ok(env('CONSUL_NSPACES_ENABLED'));
    config = {
      environment: 'test',
      CONSUL_NSPACES_ENABLED: true,
    };
    doc = {
      cookie: 'CONSUL_NSPACES_ENABLE=0',
      getElementsByTagName: makeGetElementsBy(''),
      getElementsByName: makeGetElementsBy('{}'),
      querySelector: () => makeOperatorConfig({ NamespacesEnabled: false }),
    };
    env = getEnvironment(config, win, doc);
    assert.notOk(env('CONSUL_NSPACES_ENABLED'));
  });
  test('it returns the correct dev value when set via config', function (assert) {
    let config = {
      CONSUL_NSPACES_ENABLED: true,
    };
    let env = getEnvironment(config, win, doc);
    assert.ok(env('CONSUL_NSPACES_ENABLED'));
    config = {
      CONSUL_NSPACES_ENABLED: false,
    };
    env = getEnvironment(config, win, doc);
    assert.notOk(env('CONSUL_NSPACES_ENABLED'));
  });
  test("it returns the correct dev value (ignoring cookies when the environment doesn't allow it)", function (assert) {
    let config = {
      environment: 'production',
      CONSUL_NSPACES_ENABLED: false,
    };
    let doc = {
      cookie: 'CONSUL_NSPACES_ENABLE=1',
      getElementsByTagName: makeGetElementsBy(''),
      getElementsByName: makeGetElementsBy('{}'),
      querySelector: () => makeOperatorConfig({ NamespacesEnabled: false }),
    };
    let env = getEnvironment(config, win, doc);
    assert.notOk(env('CONSUL_NSPACES_ENABLED'));
    config = {
      environment: 'production',
      CONSUL_NSPACES_ENABLED: true,
    };
    doc = {
      cookie: 'CONSUL_NSPACES_ENABLE=0',
      getElementsByTagName: makeGetElementsBy(''),
      getElementsByName: makeGetElementsBy('{}'),
      querySelector: () => makeOperatorConfig({ NamespacesEnabled: true }),
    };
    env = getEnvironment(config, win, doc);
    assert.ok(env('CONSUL_NSPACES_ENABLED'));
  });
  test('it returns the correct dev value when already set via config and is reset to false', function (assert) {
    const config = {
      environment: 'test',
    };
    const doc = {
      cookie: 'CONSUL_ACLS_ENABLE=0',
      getElementsByTagName: makeGetElementsBy(''),
      getElementsByName: makeGetElementsBy('{}'),
      querySelector: () => makeOperatorConfig({ ACLsEnabled: true }),
    };
    let env = getEnvironment(config, win, doc);
    assert.notOk(env('CONSUL_ACLS_ENABLED'));
  });
});
