/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

import { get } from 'consul-ui/tests/helpers/api';
import { HEADERS_SYMBOL as META } from 'consul-ui/utils/http/consul';

module('Integration | Serializer | <%= dasherizedModuleName %>', function(hooks) {
  setupTest(hooks);
  test('respondForQuery returns the correct data for list endpoint', function(assert) {
    const serializer = this.owner.lookup('serializer:<%= dasherizedModuleName %>');
    const dc = 'dc-1';
    const request = {
      url: `/v1/<%= dasherizedModuleName %>?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = payload.map(item =>
        Object.assign({}, item, {
          Datacenter: dc,
          uid: `["${dc}","${item.Name}"]`,
        })
      );
      const actual = serializer.respondForQuery(
        function(cb) {
          const headers = {};
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
  test('respondForQueryRecord returns the correct data for item endpoint', function(assert) {
    const serializer = this.owner.lookup('serializer:<%= dasherizedModuleName %>');
    const dc = 'dc-1';
    const id = 'slug';
    const request = {
      url: `/v1/<%= dasherizedModuleName %>/${id}?dc=${dc}`,
    };
    return get(request.url).then(function(payload) {
      const expected = {
        Datacenter: dc,
        [META]: {},
        uid: `["${dc}","${id}"]`,
      };
      const actual = serializer.respondForQueryRecord(
        function(cb) {
          const headers = {};
          const body = payload;
          return cb(headers, body);
        },
        {
          dc: dc,
          id: id,
        }
      );
      assert.deepEqual(actual, expected);
    });
  });
});
