import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Integration | Adapter | partition', function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'slug';
  test('requestForQuery returns the correct url/method', async function (assert) {
    const adapter = this.owner.lookup('adapter:partition');
    const client = this.owner.lookup('service:client/http');
    const request = function () {
      return () => client.requestParams.bind(client)(...arguments);
    };
    const expected = `GET /v1/partitions?dc=${dc}`;
    let actual = await adapter.requestForQuery(request, {
      dc: dc,
    });
    actual = actual();
    assert.equal(`${actual.method} ${actual.url}`, expected);
  });
  test('requestForQueryRecord returns the correct url/method', async function (assert) {
    const adapter = this.owner.lookup('adapter:partition');
    const client = this.owner.lookup('service:client/http');
    const request = function () {
      return () => client.requestParams.bind(client)(...arguments);
    };
    const expected = `GET /v1/partition/${id}?dc=${dc}`;
    let actual = await adapter.requestForQueryRecord(request, {
      dc: dc,
      id: id,
    });
    actual = actual();
    assert.equal(`${actual.method} ${actual.url}`, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function (assert) {
    const adapter = this.owner.lookup('adapter:partition');
    const client = this.owner.lookup('service:client/http');
    const request = function () {
      return () => client.requestParams.bind(client)(...arguments);
    };
    assert.rejects(
      adapter.requestForQueryRecord(request, {
        dc: dc,
      })
    );
  });
});
