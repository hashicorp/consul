/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Integration | Adapter | <%= dasherizedModuleName %>', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'slug';
  test('requestForQuery returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:<%= dasherizedModuleName %>');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/<%= dasherizedModuleName %>?dc=${dc}`;
    const actual = adapter.requestForQuery(client.url, {
      dc: dc,
    });
    assert.equal(actual, expected);
  });
  test('requestForQueryRecord returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:<%= dasherizedModuleName %>');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/<%= dasherizedModuleName %>/${id}?dc=${dc}`;
    const actual = adapter.requestForQueryRecord(client.url, {
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:<%= dasherizedModuleName %>');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
});
