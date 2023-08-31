/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

const assertAuthorize = function (assertion, params = {}, token, $, adapter) {
  const rpc = adapter.rpc;
  const env = adapter.env;
  const settings = adapter.settings;
  adapter.env = {
    var: (str) => $[str],
  };
  adapter.settings = {
    findBySlug: (_) => token,
  };

  adapter.rpc = function (request, respond) {
    request(
      {
        requestForAuthorize: (request, params) => {
          assertion(request, params);
        },
      },
      () => {},
      params,
      params
    );
  };
  adapter.authorize({}, { modelName: 'permission' }, 1, {});
  adapter.rpc = rpc;
  adapter.env = env;
  adapter.settings = settings;
};
module('Unit | Adapter | permission', function (hooks) {
  setupTest(hooks);

  test('it exists', function (assert) {
    let adapter = this.owner.lookup('adapter:permission');
    assert.ok(adapter);
  });

  test(`authorize adds the tokens default namespace if one isn't specified`, function (assert) {
    const adapter = this.owner.lookup('adapter:permission');
    const expected = 'test';
    const token = {
      Namespace: expected,
    };
    const env = {
      CONSUL_NSPACES_ENABLED: true,
    };
    const cases = [
      undefined,
      {
        ns: undefined,
      },
      {
        ns: '',
      },
    ];
    assert.expect(cases.length);
    cases.forEach((params) => {
      assertAuthorize(
        (request, params) => {
          assert.equal(params.ns, expected);
        },
        params,
        token,
        env,
        adapter
      );
    });
  });

  test(`authorize doesn't add the tokens default namespace if one is specified`, function (assert) {
    assert.expect(1);
    const adapter = this.owner.lookup('adapter:permission');
    const notExpected = 'test';
    const expected = 'default';
    const token = {
      Namespace: notExpected,
    };
    const env = {
      CONSUL_NSPACES_ENABLED: true,
    };
    assertAuthorize(
      (request, params) => {
        assert.equal(params.ns, expected);
      },
      {
        ns: expected,
      },
      token,
      env,
      adapter
    );
  });
  test(`authorize adds the tokens default partition if one isn't specified`, function (assert) {
    const adapter = this.owner.lookup('adapter:permission');
    const expected = 'test';
    const token = {
      Partition: expected,
    };
    const env = {
      CONSUL_PARTITIONS_ENABLED: true,
    };
    const cases = [
      undefined,
      {
        partition: undefined,
      },
      {
        partition: '',
      },
    ];
    assert.expect(cases.length);
    cases.forEach((params) => {
      assertAuthorize(
        (request, params) => {
          assert.equal(params.partition, expected);
        },
        params,
        token,
        env,
        adapter
      );
    });
  });

  test(`authorize doesn't add the tokens default partition if one is specified`, function (assert) {
    assert.expect(1);
    const adapter = this.owner.lookup('adapter:permission');
    const notExpected = 'test';
    const expected = 'default';
    const token = {
      Partition: notExpected,
    };
    const env = {
      CONSUL_PARTITIONS_ENABLED: true,
    };
    assertAuthorize(
      (request, params) => {
        assert.equal(params.partition, expected);
      },
      {
        partition: expected,
      },
      token,
      env,
      adapter
    );
  });
});
