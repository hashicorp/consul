import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
module('Integration | Adapter | intention', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'intention-name';
  test('requestForQuery returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:intention');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/connect/intentions?dc=${dc}`;
    const actual = adapter.requestForQuery(client.requestParams.bind(client), {
      dc: dc,
    });
    assert.equal(`${actual.method} ${actual.url}`, expected);
  });
  test('requestForQueryRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:intention');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/connect/intentions/${id}?dc=${dc}`;
    const actual = adapter
      .requestForQueryRecord(client.url, {
        dc: dc,
        id: id,
      })
      .split('\n')[0];
    assert.equal(actual, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:intention');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
  test('requestForCreateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:intention');
    const client = this.owner.lookup('service:client/http');
    const expected = `POST /v1/connect/intentions?dc=${dc}`;
    const actual = adapter
      .requestForCreateRecord(
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
  test('requestForUpdateRecord returns the correct url', function(assert) {
    const adapter = this.owner.lookup('adapter:intention');
    const client = this.owner.lookup('service:client/http');
    const expected = `PUT /v1/connect/intentions/${id}?dc=${dc}`;
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
    const adapter = this.owner.lookup('adapter:intention');
    const client = this.owner.lookup('service:client/http');
    const expected = `DELETE /v1/connect/intentions/${id}?dc=${dc}`;
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
});
