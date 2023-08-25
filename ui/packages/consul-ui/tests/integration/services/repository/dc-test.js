/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { setupTest } from 'ember-qunit';
import { module, skip } from 'qunit';
import repo from 'consul-ui/tests/helpers/repo';

module(`Integration | Service | dc`, function (hooks) {
  setupTest(hooks);
  skip("findBySlug (doesn't interact with the API) but still needs an int test");
  skip('findAll returns the correct data for list endpoint', function (assert) {
    const subject = this.owner.lookup('service:repository/dc');

    return repo(
      'Dc',
      'findAll',
      subject,
      function retrieveStub(stub) {
        return stub(`/v1/catalog/datacenters`, {
          CONSUL_DATACENTER_COUNT: '100',
        });
      },
      function performTest(service) {
        return service.findAll();
      },
      function performAssertion(actual, expected) {
        actual.forEach((item, i) => {
          assert.equal(actual[i].Name, item.Name);
          assert.equal(item.Local, i === 0);
        });
      }
    );
  });
});
