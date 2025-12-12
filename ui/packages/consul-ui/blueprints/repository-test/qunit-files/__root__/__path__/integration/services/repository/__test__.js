/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';

module('Integration | Repository | <%= dasherizedModuleName %>', function(hooks) {
  setupTest(hooks);

  const dc = 'dc-1';
  const id = 'slug';
  const now = Date.now();

  test('findByDatacenter returns the correct data for list endpoint', function(assert) {
    const service = this.owner.lookup('service:repository/<%= dasherizedModuleName %>');
    service.store.serializerFor('<%= dasherizedModuleName %>').timestamp = () => now;

    return repo(
      'Service',
      'findAllByDatacenter',
      service,
      function retrieveStub(stub) {
        return stub(`/v1/<%= dasherizedModuleName %>?dc=${dc}`, {
          CONSUL_<%= screamingSnakeCaseModuleName %>_COUNT: '100',
        });
      },
      function performTest(svc) {
        return svc.findAllByDatacenter(dc);
      },
      function performAssertion(actual, expected) {
        assert.deepEqual(
          actual,
          expected(function(payload) {
            return payload.map(item =>
              Object.assign({}, item, {
                SyncTime: now,
                Datacenter: dc,
                uid: `["${dc}","${item.Name}"]`,
              })
            );
          })
        );
      }
    );
  });

  test('findBySlug returns the correct data for item endpoint', function(assert) {
    const service = this.owner.lookup('service:repository/<%= dasherizedModuleName %>');

    return repo(
      'Service',
      'findBySlug',
      service,
      function retrieveStub(stub) {
        return stub(`/v1/<%= dasherizedModuleName %>/${id}?dc=${dc}`, {
          CONSUL_<%= screamingSnakeCaseModuleName %>_COUNT: 1,
        });
      },
      function performTest(svc) {
        return svc.findBySlug(id, dc);
      },
      function performAssertion(actual, expected) {
        assert.deepEqual(
          actual,
          expected(function(payload) {
            return Object.assign(
              {},
              {
                Datacenter: dc,
                uid: `["${dc}","${id}"]`,
                meta: { cursor: undefined },
              },
              payload
            );
          })
        );
      }
    );
  });
});