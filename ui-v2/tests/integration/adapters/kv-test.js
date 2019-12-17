import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | kv', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'key-name/here';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach(nspace => {
    test(`requestForQuery returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:kv');
      const client = this.owner.lookup('service:client/http');
      const expected = `GET /v1/kv/${id}?keys&dc=${dc}`;
      let actual = adapter.requestForQuery(client.url, {
        dc: dc,
        id: id,
        ns: nspace,
      });
      actual = actual.split('\n');
      assert.equal(actual.shift().trim(), expected);
      actual = actual.join('\n').trim();
      assert.equal(actual, `${typeof nspace !== 'undefined' ? `ns=${nspace}` : ``}`);
    });
    test(`requestForQueryRecord returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:kv');
      const client = this.owner.lookup('service:client/http');
      const expected = `GET /v1/kv/${id}?dc=${dc}`;
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
    test(`requestForCreateRecord returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:kv');
      const client = this.owner.lookup('service:client/http');
      const expected = `PUT /v1/kv/${id}?dc=${dc}${
        typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
      }`;
      let actual = adapter
        .requestForCreateRecord(
          client.url,
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
    test(`requestForUpdateRecord returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:kv');
      const client = this.owner.lookup('service:client/http');
      const expected = `PUT /v1/kv/${id}?dc=${dc}${
        typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
      }`;
      let actual = adapter
        .requestForUpdateRecord(
          client.url,
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
    test(`requestForDeleteRecord returns the correct url/method when the nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:kv');
      const client = this.owner.lookup('service:client/http');
      const expected = `DELETE /v1/kv/${id}?dc=${dc}${
        typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
      }`;
      let actual = adapter
        .requestForDeleteRecord(
          client.url,
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
    test(`requestForDeleteRecord returns the correct url/method for folders when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:kv');
      const client = this.owner.lookup('service:client/http');
      const folder = `${id}/`;
      const expected = `DELETE /v1/kv/${folder}?dc=${dc}${
        typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
      }&recurse`;
      let actual = adapter
        .requestForDeleteRecord(
          client.url,
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
  test("requestForQuery throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQuery(client.url, {
        dc: dc,
      });
    });
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:kv');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
});
