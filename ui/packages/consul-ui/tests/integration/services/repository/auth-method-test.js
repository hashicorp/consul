/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { setupTest } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
import { module, skip, test } from 'qunit';

module(`Integration | Service | auth-method`, function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'auth-method-name';
  const undefinedNspace = 'default';
  const undefinedPartition = 'default';
  const partition = 'default';
  [undefinedNspace, 'team-1', undefined].forEach((nspace) => {
    test(`findAllByDatacenter returns the correct data for list endpoint when nspace is ${nspace}`, function (assert) {
      const subject = this.owner.lookup('service:repository/auth-method');

      return repo(
        'auth-method',
        'findAllByDatacenter',
        subject,
        function retrieveStub(stub) {
          return stub(
            `/v1/acl/auth-methods?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
              typeof partition !== 'undefined' ? `&partition=${partition}` : ``
            }`,
            {
              CONSUL_AUTH_METHOD_COUNT: '3',
            }
          );
        },
        function performTest(service) {
          return service.findAllByDatacenter({
            dc: dc,
            nspace: nspace || undefinedNspace,
            partition: partition || undefinedPartition,
          });
        },
        function performAssertion(actual, expected) {
          assert.propContains(
            actual,
            expected(function (payload) {
              return payload.map(function (item) {
                return Object.assign({}, item, {
                  Datacenter: dc,
                  Namespace: item.Namespace || undefinedNspace,
                  Partition: item.Partition || undefinedPartition,
                  uid: `["${item.Partition || undefinedPartition}","${
                    item.Namespace || undefinedNspace
                  }","${dc}","${item.Name}"]`,
                });
              });
            })
          );
        }
      );
    });
    skip(`findBySlug returns the correct data for item endpoint when the nspace is ${nspace}`, function (assert) {
      const subject = this.owner.lookup('service:repository/auth-method');

      return repo(
        'AuthMethod',
        'findBySlug',
        subject,
        function retrieveStub(stub) {
          return stub(
            `/v1/acl/auth-method/${id}?dc=${dc}${
              typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
            }`
          );
        },
        function performTest(service) {
          return service.findBySlug(id, dc, nspace || undefinedNspace);
        },
        function performAssertion(actual, expected) {
          assert.propContains(
            actual,
            expected(function (payload) {
              const item = payload;
              return Object.assign({}, item, {
                Datacenter: dc,
                Namespace: item.Namespace || undefinedNspace,
                uid: `["${item.Namespace || undefinedNspace}","${dc}","${item.Name}"]`,
                meta: {
                  cacheControl: undefined,
                  cursor: undefined,
                  dc: dc,
                  nspace: item.Namespace || undefinedNspace,
                },
              });
            })
          );
        }
      );
    });
  });
});
