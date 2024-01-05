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

module('Integration | Serializer | oidc-provider', function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const undefinedNspace = 'default';
  const undefinedPartition = 'default';
  const partition = 'default';
  [undefinedNspace, 'team-1', undefined].forEach((nspace) => {
    test(`respondForQuery returns the correct data for list endpoint when the nspace is ${nspace}`, function (assert) {
      assert.expect(1);

      const serializer = this.owner.lookup('serializer:oidc-provider');
      const request = {
        url: `/v1/internal/ui/oidc-auth-methods?dc=${dc}`,
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
          }
        );
        assert.deepEqual(actual, expected);
      });
    });
    test(`respondForQueryRecord returns the correct data for item endpoint when the nspace is ${nspace}`, function (assert) {
      assert.expect(1);

      const serializer = this.owner.lookup('serializer:oidc-provider');
      const dc = 'dc-1';
      const id = 'slug';
      const request = {
        url: `/v1/acl/oidc/auth-url?dc=${dc}${
          typeof nspace !== 'undefined' ? `&ns=${nspace || undefinedNspace}` : ``
        }${typeof partition !== 'undefined' ? `&partition=${partition || undefinedNspace}` : ``}`,
      };
      return get(request.url).then(function (payload) {
        // The response here never has a Namespace property so its ok to just
        // use the query parameter as the expected nspace value. See
        // implementation of this method for info on why this is slightly
        // different to other tests
        const expected = Object.assign({}, payload, {
          Name: id,
          Datacenter: dc,
          [META]: {
            [DC.toLowerCase()]: dc,
            [NSPACE.toLowerCase()]: nspace || undefinedNspace,
            [PARTITION.toLowerCase()]: partition || undefinedPartition,
          },
          Namespace: nspace || undefinedNspace,
          Partition: partition || undefinedPartition,
          uid: `["${partition || undefinedPartition}","${
            nspace || undefinedNspace
          }","${dc}","${id}"]`,
        });
        const actual = serializer.respondForQueryRecord(
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
            id: id,
            ns: nspace,
            partition: partition,
          }
        );
        assert.deepEqual(actual, expected);
      });
    });
  });
});
