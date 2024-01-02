/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { env } from '../../../env';
const shouldHaveNspace = function (nspace) {
  return typeof nspace !== 'undefined' && env('CONSUL_NSPACES_ENABLED');
};

module('Integration | Adapter | permission', function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach((nspace) => {
    test('requestForAuthorize returns the correct url/method', function (assert) {
      const adapter = this.owner.lookup('adapter:permission');
      const client = this.owner.lookup('service:client/http');
      const request = client.requestParams.bind(client);
      // authorize endpoint doesn't need an ns sending on the query param
      const expected = `POST /v1/internal/acl/authorize?dc=${dc}${
        shouldHaveNspace(nspace) ? `` : ``
      }`;
      const actual = adapter.requestForAuthorize(request, {
        dc: dc,
        ns: nspace,
      });
      assert.equal(`${actual.method} ${actual.url}`, expected);
    });
  });
});
