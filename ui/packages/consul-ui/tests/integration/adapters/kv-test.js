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
module('Integration | Adapter | kv', function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'key-name/here';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach((nspace) => {
    test(`requestForQuery returns the correct url/method when nspace is ${nspace}`, async function (assert) {
      const adapter = this.owner.lookup('adapter:kv');
      const client = this.owner.lookup('service:client/http');
      const request = function () {
        return () => client.requestParams.bind(client)(...arguments);
      };
      const expected = `GET /v1/kv/${id}?keys&dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      let actual = await adapter.requestForQuery(request, {
        dc: dc,
        id: id,
        ns: nspace,
      });
      actual = actual();
      assert.equal(`${actual.method} ${actual.url}`, expected);
    });
    test(`requestForQueryRecord returns the correct url/method when nspace is ${nspace}`, async function (assert) {
      const adapter = this.owner.lookup('adapter:kv');
      const client = this.owner.lookup('service:client/http');
      const request = function () {
        return () => client.requestParams.bind(client)(...arguments);
      };
      const expected = `GET /v1/kv/${id}?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      let actual = await adapter.requestForQueryRecord(request, {
        dc: dc,
        id: id,
        ns: nspace,
      });
      actual = actual();
      assert.equal(`${actual.method} ${actual.url}`, expected);
    });
    test(`requestForCreateRecord returns the correct url/method when nspace is ${nspace}`, function (assert) {
      const adapter = this.owner.lookup('adapter:kv');
      const client = this.owner.lookup('service:client/http');
      const request = client.url.bind(client);
      const expected = `PUT /v1/kv/${id}?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      let actual = adapter
        .requestForCreateRecord(
          request,
          {},
          {
            Datacenter: dc,
            Key: id,
            Value: '',
            Namespace: nspace,
          }
        )
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
    test(`requestForUpdateRecord returns the correct url/method when nspace is ${nspace}`, function (assert) {
      const adapter = this.owner.lookup('adapter:kv');
      const client = this.owner.lookup('service:client/http');
      const request = client.url.bind(client);
      const flags = 12;
      const expected = `PUT /v1/kv/${id}?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }&flags=${flags}`;
      let actual = adapter
        .requestForUpdateRecord(
          request,
          {},
          {
            Datacenter: dc,
            Key: id,
            Value: '',
            Namespace: nspace,
            Flags: flags,
          }
        )
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
    test(`requestForDeleteRecord returns the correct url/method when the nspace is ${nspace}`, function (assert) {
      const adapter = this.owner.lookup('adapter:kv');
      const client = this.owner.lookup('service:client/http');
      const request = client.url.bind(client);
      const expected = `DELETE /v1/kv/${id}?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      let actual = adapter
        .requestForDeleteRecord(
          request,
          {},
          {
            Datacenter: dc,
            Key: id,
            Namespace: nspace,
          }
        )
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
    test(`requestForDeleteRecord returns the correct url/method for folders when nspace is ${nspace}`, function (assert) {
      const adapter = this.owner.lookup('adapter:kv');
      const client = this.owner.lookup('service:client/http');
      const request = client.url.bind(client);
      const folder = `${id}/`;
      const expected = `DELETE /v1/kv/${folder}?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }&recurse`;
      let actual = adapter
        .requestForDeleteRecord(
          request,
          {},
          {
            Datacenter: dc,
            Key: folder,
            Namespace: nspace,
          }
        )
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
  });
  test("requestForQuery throws if you don't specify an id", function (assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    assert.rejects(
      adapter.requestForQuery(request, {
        dc: dc,
      })
    );
  });
  test("requestForQueryRecord throws if you don't specify an id", function (assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    assert.rejects(
      adapter.requestForQueryRecord(request, {
        dc: dc,
      })
    );
  });
});
