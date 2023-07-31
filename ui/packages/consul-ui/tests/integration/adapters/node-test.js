import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { env } from '../../../env';
const shouldHaveNspace = function (nspace) {
  return typeof nspace !== 'undefined' && env('CONSUL_NSPACES_ENABLED');
};
module('Integration | Adapter | node', function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'node-name';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach((nspace) => {
    test(`requestForQuery returns the correct url when nspace is ${nspace}`, function (assert) {
      const adapter = this.owner.lookup('adapter:node');
      const client = this.owner.lookup('service:client/http');
      const request = client.requestParams.bind(client);
      const expected = `GET /v1/internal/ui/nodes?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      const actual = adapter.requestForQuery(request, {
        dc: dc,
        ns: nspace,
      });
      assert.equal(`${actual.method} ${actual.url}`, expected);
    });
    test(`requestForQueryRecord returns the correct url when the nspace is ${nspace}`, function (assert) {
      const adapter = this.owner.lookup('adapter:node');
      const client = this.owner.lookup('service:client/http');
      const request = client.requestParams.bind(client);
      const expected = `GET /v1/internal/ui/node/${id}?dc=${dc}${
        shouldHaveNspace(nspace) ? `&ns=${nspace}` : ``
      }`;
      const actual = adapter.requestForQueryRecord(request, {
        dc: dc,
        id: id,
        ns: nspace,
      });
      assert.equal(`${actual.method} ${actual.url}`, expected);
    });
  });
  // the following don't require nspacing
  test("requestForQueryRecord throws if you don't specify an id", function (assert) {
    const adapter = this.owner.lookup('adapter:node');
    const client = this.owner.lookup('service:client/http');
    const request = client.requestParams.bind(client);
    assert.throws(function () {
      adapter.requestForQueryRecord(request, {
        dc: dc,
      });
    });
  });
  test('requestForQueryLeader returns the correct url', function (assert) {
    const adapter = this.owner.lookup('adapter:node');
    const client = this.owner.lookup('service:client/http');
    const request = client.requestParams.bind(client);
    const expected = `GET /v1/status/leader?dc=${dc}`;
    const actual = adapter.requestForQueryLeader(request, {
      dc: dc,
    });
    assert.equal(`${actual.method} ${actual.url}`, expected);
  });
});
