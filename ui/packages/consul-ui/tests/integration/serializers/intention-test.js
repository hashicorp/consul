/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
module('Integration | Serializer | intention', function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'intention-name';
  const nspace = 'default';
  const partition = 'default';
  test('respondForQuery returns the correct data for list endpoint', function (assert) {
    assert.expect(4);
    const serializer = this.owner.lookup('serializer:intention');
    const request = {
      url: `/v1/connect/intentions?dc=${dc}`,
    };
    return get(request.url).then(function (payload) {
      const expected = payload.map((item) => {
        if (item.SourcePeer) {
          delete item.SourcePeer;
        }
        return Object.assign({}, item, {
          Datacenter: dc,
          // TODO: default isn't required here, once we've
          // refactored out our Serializer this can go
          Namespace: nspace,
          Partition: partition,
          uid: `["${partition}","${nspace}","${dc}","${item.SourcePartition}:${item.SourceNS}:${item.SourceName}:${item.DestinationPartition}:${item.DestinationNS}:${item.DestinationName}"]`,
        });
      });
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
        }
      );
      assert.equal(actual[0].Partition, expected[0].Partition);
      assert.equal(actual[0].Namespace, expected[0].Namespace);
      assert.equal(actual[0].Datacenter, expected[0].Datacenter);
      assert.equal(actual[0].uid, expected[0].uid);
    });
  });
  test('respondForQueryRecord returns the correct data for item endpoint', function (assert) {
    assert.expect(4);
    const serializer = this.owner.lookup('serializer:intention');
    const request = {
      url: `/v1/connect/intentions/${id}?dc=${dc}`,
    };
    const item = {
      SourceName: 'SourceName',
      DestinationName: 'DestinationName',
      SourceNS: 'SourceNS',
      DestinationNS: 'DestinationNS',
      SourcePartition: 'SourcePartition',
      DestinationPartition: 'DestinationPartition',
    };
    return get(request.url).then(function (payload) {
      payload = {
        ...payload,
        ...item,
      };
      const expected = Object.assign({}, payload, {
        Datacenter: dc,
        [META]: {
          [DC.toLowerCase()]: dc,
          [NSPACE.toLowerCase()]: nspace,
          [PARTITION.toLowerCase()]: partition,
        },
        // TODO: default isn't required here, once we've
        // refactored out our Serializer this can go
        Namespace: nspace,
        Partition: partition,
        uid: `["${partition}","${nspace}","${dc}","${item.SourcePartition}:${item.SourceNS}:${item.SourceName}:${item.DestinationPartition}:${item.DestinationNS}:${item.DestinationName}"]`,
      });
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
        }
      );
      assert.equal(actual.Partition, expected.Partition);
      assert.equal(actual.Namespace, expected.Namespace);
      assert.equal(actual.Datacenter, expected.Datacenter);
      assert.equal(actual.uid, expected.uid);
    });
  });
});
