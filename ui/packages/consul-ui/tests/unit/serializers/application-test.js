/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { HEADERS_SYMBOL as META } from 'consul-ui/utils/http/consul';
import Node from 'consul-ui/models/node';

module('Unit | Serializer | application', function (hooks) {
  setupTest(hooks);

  // Replace this with your real tests.
  test('it exists', function (assert) {
    const store = this.owner.lookup('service:store');
    const serializer = store.serializerFor('application');

    assert.ok(serializer);
  });
  test('respondForDeleteRecord returns the expected pojo structure', function (assert) {
    const store = this.owner.lookup('service:store');
    const serializer = store.serializerFor('application');
    serializer.primaryKey = 'primary-key-name';
    serializer.slugKey = 'Name';
    serializer.fingerprint = function (primary, slug, foreignValue) {
      return function (item) {
        return {
          ...item,
          ...{
            Datacenter: foreignValue,
            [primary]: item[slug],
          },
        };
      };
    };
    // adapter.uidForURL = this.stub().returnsArg(0);
    const respond = function (cb) {
      const headers = {};
      const body = true;
      return cb(headers, body);
    };
    const expected = {
      'primary-key-name': 'name',
    };
    const actual = serializer.respondForDeleteRecord(respond, {}, { Name: 'name', dc: 'dc-1' });
    assert.deepEqual(actual, expected);
    // assert.ok(adapter.uidForURL.calledOnce);
  });
  test('respondForQueryRecord returns the expected pojo structure', function (assert) {
    const store = this.owner.lookup('service:store');
    const serializer = store.serializerFor('application');
    serializer.primaryKey = 'primary-key-name';
    serializer.slugKey = 'Name';
    serializer.fingerprint = function (primary, slug, foreignValue) {
      return function (item) {
        return {
          ...item,
          ...{
            Datacenter: foreignValue,
            [primary]: item[slug],
          },
        };
      };
    };
    const expected = {
      Datacenter: 'dc-1',
      Name: 'name',
      [META]: {},
      'primary-key-name': 'name',
    };
    const respond = function (cb) {
      const headers = {};
      const body = {
        Name: 'name',
      };
      return cb(headers, body);
    };
    const actual = serializer.respondForQueryRecord(respond, { Name: 'name', dc: 'dc-1' });
    assert.deepEqual(actual, expected);
  });
  test('respondForQuery returns the expected pojo structure', function (assert) {
    const store = this.owner.lookup('service:store');
    const serializer = store.serializerFor('application');
    serializer.primaryKey = 'primary-key-name';
    serializer.slugKey = 'Name';
    serializer.fingerprint = function (primary, slug, foreignValue) {
      return function (item) {
        return {
          ...item,
          ...{
            Datacenter: foreignValue,
            [primary]: item[slug],
          },
        };
      };
    };
    const expected = [
      {
        Datacenter: 'dc-1',
        Name: 'name1',
        'primary-key-name': 'name1',
      },
      {
        Datacenter: 'dc-1',
        Name: 'name2',
        'primary-key-name': 'name2',
      },
    ];
    const respond = function (cb) {
      const headers = {};
      const body = [
        {
          Name: 'name1',
        },
        {
          Name: 'name2',
        },
      ];
      return cb(headers, body);
    };
    const actual = serializer.respondForQuery(respond, { Name: 'name', dc: 'dc-1' });
    assert.deepEqual(actual, expected);
    // assert.ok(adapter.uidForURL.calledTwice);
  });
  test('normalizeResponse for Node returns the expected meta in response', function (assert) {
    const store = this.owner.lookup('service:store');
    const serializer = store.serializerFor('application');
    serializer.timestamp = () => 1234567890; //mocks actual timestamp
    serializer.primaryKey = 'primary-key-name';
    serializer.slugKey = 'Name';
    serializer.fingerprint = function (primary, slug, foreignValue) {
      return function (item) {
        return {
          ...item,
          ...{
            Datacenter: foreignValue,
            [primary]: item[slug],
          },
        };
      };
    };

    const payload = [
      {
        Node: 'node-0',
        Meta: { 'consul-version': '1.7.2' },
        uid: '1234',
        SyncTime: 1234567890,
      },
      {
        Node: 'node-1',
        Meta: { 'consul-version': '1.18.0' },
        uid: '1235',
        SyncTime: 1234567891,
      },
      // synthetic-node without consul-version meta
      {
        Node: 'node-2',
        Meta: { 'synthetic-node': true },
        uid: '1236',
        SyncTime: 1234567891,
      },
    ];

    const expected = {
      data: [
        {
          attributes: {
            Node: 'node-0',
            Meta: { 'consul-version': '1.7.2' },
            SyncTime: 1234567890,
            uid: '1234',
          },
          id: '1234',
          relationships: {},
          type: 'node',
        },
        {
          attributes: {
            Node: 'node-1',
            Meta: { 'consul-version': '1.18.0' },
            SyncTime: 1234567890,
            uid: '1235',
          },
          id: '1235',
          relationships: {},
          type: 'node',
        },
        {
          attributes: {
            Node: 'node-2',
            Meta: { 'synthetic-node': true },
            SyncTime: 1234567890,
            uid: '1236',
          },
          id: '1236',
          relationships: {},
          type: 'node',
        },
      ],
      included: [],
      meta: {
        versions: ['1.18', '1.7'], //expect distinct major versions sorted
        cacheControl: undefined,
        cursor: undefined,
        date: 1234567890,
        dc: undefined,
        nspace: undefined,
        partition: undefined,
      },
    };
    const actual = serializer.normalizeResponse(store, Node, payload, '2', 'query');
    assert.deepEqual(actual, expected);
  });
});
