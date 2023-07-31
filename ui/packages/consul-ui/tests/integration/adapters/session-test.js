import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { env } from '../../../env';
const shouldHaveNspace = function (nspace) {
  return typeof nspace !== 'undefined' && env('CONSUL_NSPACES_ENABLED');
};
module('Integration | Adapter | session', function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'session-id';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach((nspace) => {
    test(`requestForQuery returns the correct url/method when nspace is ${nspace}`, function (assert) {
      const adapter = this.owner.lookup('adapter:session');
      const client = this.owner.lookup('service:client/http');
      const request = client.requestParams.bind(client);
      const node = 'node-id';
      const expected = `GET /v1/session/node/${node}?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      let actual = adapter.requestForQuery(request, {
        dc: dc,
        id: node,
        ns: nspace,
      });
      assert.equal(`${actual.method} ${actual.url}`, expected);
    });
    test(`requestForQueryRecord returns the correct url/method when nspace is ${nspace}`, function (assert) {
      const adapter = this.owner.lookup('adapter:session');
      const client = this.owner.lookup('service:client/http');
      const request = client.requestParams.bind(client);
      const expected = `GET /v1/session/info/${id}?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      let actual = adapter.requestForQueryRecord(request, {
        dc: dc,
        id: id,
        ns: nspace,
      });
      assert.equal(`${actual.method} ${actual.url}`, expected);
    });
    test(`requestForDeleteRecord returns the correct url/method when the nspace is ${nspace}`, function (assert) {
      const adapter = this.owner.lookup('adapter:session');
      const client = this.owner.lookup('service:client/http');
      const request = client.url.bind(client);
      const expected = `PUT /v1/session/destroy/${id}?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      const actual = adapter
        .requestForDeleteRecord(
          request,
          {},
          {
            Datacenter: dc,
            ID: id,
            Namespace: nspace,
          }
        )
        .split('\n')
        .shift();
      assert.equal(actual, expected);
    });
  });
  test("requestForQuery throws if you don't specify an id", function (assert) {
    const adapter = this.owner.lookup('adapter:session');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    assert.throws(function () {
      adapter.requestForQuery(request, {
        dc: dc,
      });
    });
  });
  test("requestForQueryRecord throws if you don't specify an id", function (assert) {
    const adapter = this.owner.lookup('adapter:session');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    assert.throws(function () {
      adapter.requestForQueryRecord(request, {
        dc: dc,
      });
    });
  });
});
