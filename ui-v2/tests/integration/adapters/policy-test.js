import { module, test, skip } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | policy', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'policy-name';
  test('requestForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:policy');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/acl/policies?dc=${dc}`;
    const actual = adapter.requestForQuery(client.url, {
      dc: dc,
    });
    assert.equal(actual, expected);
  });
  test('requestForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:policy');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/acl/policy/${id}?dc=${dc}`;
    const actual = adapter.requestForQueryRecord(client.url, {
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:policy');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
  test('requestForCreateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:policy');
    const client = this.owner.lookup('service:client/http');
    const expected = `PUT /v1/acl/policy?dc=${dc}`;
    const actual = adapter
      .requestForCreateRecord(
        client.url,
        {},
        {
          Datacenter: dc,
        }
      )
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  test('requestForUpdateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:policy');
    const client = this.owner.lookup('service:client/http');
    const expected = `PUT /v1/acl/policy/${id}?dc=${dc}`;
    const actual = adapter
      .requestForUpdateRecord(
        client.url,
        {},
        {
          Datacenter: dc,
          ID: id,
        }
      )
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  test('requestForDeleteRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:policy');
    const client = this.owner.lookup('service:client/http');
    const expected = `DELETE /v1/acl/policy/${id}?dc=${dc}`;
    const actual = adapter
      .requestForDeleteRecord(
        client.url,
        {},
        {
          Datacenter: dc,
          ID: id,
        }
      )
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  skip('urlForTranslateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:policy');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/acl/policy/translate`;
    const actual = adapter.requestForTranslateRecord(client.id, {});
    assert.equal(actual, expected);
  });
});
