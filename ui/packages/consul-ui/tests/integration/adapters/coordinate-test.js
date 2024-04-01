/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | coordinate', function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  test('requestForQuery returns the correct url', function (assert) {
    const adapter = this.owner.lookup('adapter:coordinate');
    const client = this.owner.lookup('service:client/http');
    const request = client.requestParams.bind(client);
    const expected = `GET /v1/coordinate/nodes?dc=${dc}`;
    const actual = adapter.requestForQuery(request, {
      dc: dc,
    });
    assert.equal(`${actual.method} ${actual.url}`, expected);
  });
  test('requestForQuery returns the correct body', function (assert) {
    const adapter = this.owner.lookup('adapter:coordinate');
    const client = this.owner.lookup('service:client/http');
    const request = client.body.bind(client);
    const expected = {
      index: 1,
    };
    const [actual] = adapter.requestForQuery(request, {
      dc: dc,
      index: 1,
    });
    assert.deepEqual(actual, expected);
  });
});
