/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import {
  HEADERS_SYMBOL as META,
  HEADERS_DATACENTER as DC,
  HEADERS_NAMESPACE as NSPACE,
  HEADERS_PARTITION as PARTITION,
} from 'consul-ui/utils/http/consul';
module('Integration | Serializer | node', function (hooks) {
  setupTest(hooks);
  const nspace = 'default';
  const partition = 'default';
  test('respondForQuery returns the correct data for list endpoint', function (assert) {
    assert.expect(4);
    const store = this.owner.lookup('service:store');
    const serializer = this.owner.lookup('serializer:node');
    serializer.store = store;
    const modelClass = store.modelFor('node');
    const dc = 'dc-1';
    const request = {
      url: `/v1/internal/ui/nodes?dc=${dc}`,
    };
    return get(request.url).then(function (payload) {
      const actual = serializer.respondForQuery(
        function (cb) {
          const headers = {
            [DC]: dc,
            [NSPACE]: nspace,
            [PARTITION]: partition,
          };
          const body = payload;
          return cb(headers, body);
        },
        {
          dc: dc,
        },
        {
          dc: dc,
        },
        modelClass
      );
      assert.equal(actual[0].Datacenter, dc);
      assert.equal(actual[0].Namespace, nspace);
      assert.equal(actual[0].Partition, partition);
      assert.equal(actual[0].uid, `["${partition}","${nspace}","${dc}","${actual[0].ID}"]`);
    });
  });
  test('respondForQueryRecord returns the correct data for item endpoint', function (assert) {
    assert.expect(4);
    const store = this.owner.lookup('service:store');
    const serializer = this.owner.lookup('serializer:node');
    serializer.store = store;
    const modelClass = store.modelFor('node');
    const dc = 'dc-1';
    const id = 'node-name';
    const request = {
      url: `/v1/internal/ui/node/${id}?dc=${dc}`,
    };
    return get(request.url).then(function (payload) {
      const actual = serializer.respondForQueryRecord(
        function (cb) {
          const headers = {
            [DC]: dc,
            [NSPACE]: nspace,
            [PARTITION]: partition,
          };
          const body = payload;
          return cb(headers, body);
        },
        {
          dc: dc,
        },
        {
          dc: dc,
        },
        modelClass
      );
      assert.equal(actual.Datacenter, dc);
      assert.equal(actual.Namespace, nspace);
      assert.equal(actual.Partition, partition);
      assert.equal(actual.uid, `["${partition}","${nspace}","${dc}","${actual.ID}"]`);
    });
  });
  test('respondForQueryLeader returns the correct data', function (assert) {
    assert.expect(1);

    const serializer = this.owner.lookup('serializer:node');
    const dc = 'dc-1';
    const request = {
      url: `/v1/status/leader?dc=${dc}`,
    };
    return get(request.url).then(function (payload) {
      const expected = {
        Address: '211.245.86.75',
        Port: '8500',
        [META]: {
          [DC.toLowerCase()]: dc,
          [NSPACE.toLowerCase()]: nspace,
        },
      };
      const actual = serializer.respondForQueryLeader(
        function (cb) {
          const headers = {
            [DC]: dc,
            [NSPACE]: nspace,
          };
          const body = payload;
          return cb(headers, body);
        },
        {
          dc: dc,
        }
      );
      assert.deepEqual(actual, expected);
    });
  });
});
