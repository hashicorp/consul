/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import {
  HEADERS_DATACENTER as DC,
  HEADERS_NAMESPACE as NSPACE,
  HEADERS_PARTITION as PARTITION,
} from 'consul-ui/utils/http/consul';
module('Integration | Serializer | service', function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const undefinedNspace = 'default';
  const undefinedPartition = 'default';
  const partition = 'default';
  [undefinedNspace, 'team-1', undefined].forEach((nspace) => {
    test(`respondForQuery returns the correct data for list endpoint when nspace is ${nspace}`, function (assert) {
      assert.expect(4);
      const serializer = this.owner.lookup('serializer:service');
      const request = {
        url: `/v1/internal/ui/services?dc=${dc}${
          typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
        }${typeof partition !== 'undefined' ? `&partition=${partition}` : ``}`,
      };
      return get(request.url).then(function (payload) {
        const expected = payload.map((item) =>
          Object.assign({}, item, {
            Namespace: item.Namespace || undefinedNspace,
            Datacenter: dc,
            Partition: item.Partition || undefinedPartition,
            uid: `["${item.Partition || undefinedPartition}","${
              item.Namespace || undefinedNspace
            }","${dc}","${item.Name}"]`,
          })
        );
        const actual = serializer.respondForQuery(
          function (cb) {
            const headers = {
              [DC]: dc,
              [NSPACE]: nspace || undefinedNspace,
              [PARTITION]: partition || undefinedPartition,
            };
            const body = payload;
            return cb(headers, body);
          },
          {
            dc: dc,
            ns: nspace,
            partition: partition || undefinedPartition,
          }
        );
        assert.equal(actual[0].Namespace, expected[0].Namespace);
        assert.equal(actual[0].Partition, expected[0].Partition);
        assert.equal(actual[0].Datacenter, expected[0].Datacenter);
        assert.equal(actual[0].uid, expected[0].uid);
      });
    });
    test(`respondForQuery returns the correct data for list endpoint when gateway is set when nspace is ${nspace}`, function (assert) {
      assert.expect(1);

      const serializer = this.owner.lookup('serializer:service');
      const gateway = 'gateway';
      const request = {
        url: `/v1/internal/ui/gateway-services-nodes/${gateway}?dc=${dc}${
          typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
        }${typeof partition !== 'undefined' ? `&partition=${partition}` : ``}`,
      };
      return get(request.url).then(function (payload) {
        const expected = payload.map((item) =>
          Object.assign({}, item, {
            Datacenter: dc,
            Namespace: item.Namespace || undefinedNspace,
            Partition: item.Partition || undefinedPartition,
            uid: `["${item.Partition || undefinedPartition}","${
              item.Namespace || undefinedNspace
            }","${dc}","${item.Name}"]`,
          })
        );
        const actual = serializer.respondForQuery(
          function (cb) {
            const headers = {
              [DC]: dc,
              [NSPACE]: nspace || undefinedNspace,
              [PARTITION]: partition || undefinedPartition,
            };
            const body = payload;
            return cb(headers, body);
          },
          {
            dc: dc,
            ns: nspace,
            partition: partition || undefinedPartition,
            gateway: gateway,
          }
        );
        assert.deepEqual(actual, expected);
      });
    });
  });
});
