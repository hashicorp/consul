import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | session', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'session-id';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach(nspace => {
    test(`requestForQuery returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:session');
      const client = this.owner.lookup('service:client/http');
      const node = 'node-id';
      const expected = `GET /v1/session/node/${node}?dc=${dc}`;
      let actual = adapter.requestForQuery(client.url, {
        dc: dc,
        id: node,
        ns: nspace,
      });
      actual = actual.split('\n');
      assert.equal(actual.shift().trim(), expected);
      actual = actual.join('\n').trim();
      assert.equal(actual, `${typeof nspace !== 'undefined' ? `ns=${nspace}` : ``}`);
    });
    test(`requestForQueryRecord returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:session');
      const client = this.owner.lookup('service:client/http');
      const expected = `GET /v1/session/info/${id}?dc=${dc}`;
      let actual = adapter.requestForQueryRecord(client.url, {
        dc: dc,
        id: id,
        ns: nspace,
      });
      actual = actual.split('\n');
      assert.equal(actual.shift().trim(), expected);
      actual = actual.join('\n').trim();
      assert.equal(actual, `${typeof nspace !== 'undefined' ? `ns=${nspace}` : ``}`);
    });
    test(`requestForDeleteRecord returns the correct url/method when the nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:session');
      const client = this.owner.lookup('service:client/http');
      const expected = `PUT /v1/session/destroy/${id}?dc=${dc}${
        typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
      }`;
      const actual = adapter
        .requestForDeleteRecord(
          client.url,
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
  test("requestForQuery throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:session');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQuery(client.url, {
        dc: dc,
      });
    });
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:session');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
});
