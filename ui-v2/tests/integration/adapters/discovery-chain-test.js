import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import getNspaceRunner from 'consul-ui/tests/helpers/get-nspace-runner';

const nspaceRunner = getNspaceRunner('discovery-chain');
module('Integration | Adapter | discovery-chain', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'slug';
  test('requestForQueryRecord returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:discovery-chain');
    const client = this.owner.lookup('service:client/http');
    const expected = `GET /v1/discovery-chain/${id}?dc=${dc}`;
    const actual = adapter.requestForQueryRecord(client.url, {
      dc: dc,
      id: id,
    });
    assert.equal(actual, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:discovery-chain');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
  test('requestForQueryRecord returns the correct body', function(assert) {
    return nspaceRunner(
      (adapter, serializer, client) => {
        return adapter.requestForQueryRecord(client.body, {
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
