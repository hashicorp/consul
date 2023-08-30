/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';

module(`Integration | Service | service`, function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const now = new Date().getTime();
  const undefinedNspace = 'default';
  const undefinedPartition = 'default';
  const partition = 'default';
  [undefinedNspace, 'team-1', undefined].forEach((nspace) => {
    test(`findGatewayBySlug returns the correct data for list endpoint when nspace is ${nspace}`, function (assert) {
      assert.expect(5);

      const subject = this.owner.lookup('service:repository/service');
      subject.store.serializerFor('service').timestamp = function () {
        return now;
      };
      const gateway = 'gateway';
      const conf = {
        cursor: 1,
      };
      return repo(
        'Service',
        'findGatewayBySlug',
        subject,
        function retrieveStub(stub) {
          return stub(
            `/v1/internal/ui/gateway-services-nodes/${gateway}?dc=${dc}${
              typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
            }${typeof partition !== 'undefined' ? `&partition=${partition}` : ``}`,
            {
              CONSUL_SERVICE_COUNT: '100',
            }
          );
        },
        function performTest(service) {
          return service.findGatewayBySlug(
            {
              gateway,
              dc,
              ns: nspace || undefinedNspace,
              partition: partition || undefinedPartition,
            },
            conf
          );
        },
        function performAssertion(actual, expected) {
          const result = expected(function (payload) {
            return payload.map((item) =>
              Object.assign({}, item, {
                SyncTime: now,
                Datacenter: dc,
                Namespace: item.Namespace || undefinedNspace,
                Partition: item.Partition || undefinedPartition,
                uid: `["${item.Partition || undefinedPartition}","${
                  item.Namespace || undefinedNspace
                }","${dc}","${item.Name}"]`,
              })
            );
          });
          assert.equal(actual[0].SyncTime, result[0].SyncTime);
          assert.equal(actual[0].Datacenter, result[0].Datacenter);
          assert.equal(actual[0].Namespace, result[0].Namespace);
          assert.equal(actual[0].Partition, result[0].Partition);
          assert.equal(actual[0].uid, result[0].uid);
        }
      );
    });
  });
});
