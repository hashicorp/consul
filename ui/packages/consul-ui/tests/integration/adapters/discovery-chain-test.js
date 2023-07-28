import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import getNspaceRunner from 'consul-ui/tests/helpers/get-nspace-runner';

const nspaceRunner = getNspaceRunner('discovery-chain');
module('Integration | Adapter | discovery-chain', function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'slug';
  test('requestForQueryRecord returns the correct url/method', function (assert) {
    const adapter = this.owner.lookup('adapter:discovery-chain');
    const client = this.owner.lookup('service:client/http');
    const request = client.requestParams.bind(client);
    const expected = `GET /v1/discovery-chain/${id}?dc=${dc}`;
    const actual = adapter.requestForQueryRecord(request, {
      dc: dc,
      id: id,
    });
    assert.equal(`${actual.method} ${actual.url}`, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function (assert) {
    const adapter = this.owner.lookup('adapter:discovery-chain');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    assert.throws(function () {
      adapter.requestForQueryRecord(request, {
        dc: dc,
      });
    });
  });
  test('requestForQueryRecord returns the correct body', function (assert) {
    return nspaceRunner(
      (adapter, serializer, client) => {
        const request = client.body.bind(client);
        return adapter.requestForQueryRecord(request, {
          id: id,
          dc: dc,
          ns: 'team-1',
          index: 1,
        });
      },
      {
        index: 1,
        ns: 'team-1',
      },
      {
        index: 1,
      },
      this,
      assert
    );
  });
});
