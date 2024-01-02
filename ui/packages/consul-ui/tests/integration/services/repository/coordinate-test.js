/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { setupTest } from 'ember-qunit';
import { module, test } from 'qunit';
import repo from 'consul-ui/tests/helpers/repo';

const dc = 'dc-1';
const nspace = 'default';
const partition = 'default';
const now = new Date().getTime();
module(`Integration | Service | coordinate`, function (hooks) {
  setupTest(hooks);

  test('findAllByDatacenter returns the correct data for list endpoint', function (assert) {
    assert.expect(1);

    const subject = this.owner.lookup('service:repository/coordinate');

    subject.store.serializerFor('coordinate').timestamp = function () {
      return now;
    };
    return repo(
      'Coordinate',
      'findAllByDatacenter',
      subject,
      function retrieveStub(stub) {
        return stub(
          `/v1/coordinate/nodes?dc=${dc}${
            typeof partition !== 'undefined' ? `&partition=${partition}` : ``
          }`,
          {
            CONSUL_NODE_COUNT: '100',
          }
        );
      },
      function performTest(service) {
        return service.findAllByDatacenter({ dc, partition });
      },
      function performAssertion(actual, expected) {
        assert.deepEqual(
          actual,
          expected(function (payload) {
            return payload.map((item) =>
              Object.assign({}, item, {
                SyncTime: now,
                Datacenter: dc,
                Partition: partition,
                // TODO: nspace isn't required here, once we've
                // refactored out our Serializer this can go
                uid: `["${partition}","${nspace}","${dc}","${item.Node}"]`,
              })
            );
          })
        );
      }
    );
  });
  test('findAllByNode calls findAllByDatacenter with the correct arguments', function (assert) {
    assert.expect(3);

    const datacenter = 'dc-1';
    const conf = {
      cursor: 1,
    };
    const service = this.owner.lookup('service:repository/coordinate');
    service.findAllByDatacenter = function (params, configuration) {
      assert.equal(
        arguments.length,
        2,
        'Expected to be called with the correct number of arguments'
      );
      assert.equal(params.dc, datacenter);
      assert.deepEqual(configuration, conf);
      return Promise.resolve([]);
    };
    return service.findAllByNode({ node: 'node-name', dc: datacenter }, conf);
  });
});
