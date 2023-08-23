import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import getNspaceRunner from 'consul-ui/tests/helpers/get-nspace-runner';

const nspaceRunner = getNspaceRunner('topology');
module('Integration | Adapter | topology', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'slug';
  const kind = '';
  test('requestForQueryRecord returns the correct url/method', function(assert) {
    const adapter = this.owner.lookup('adapter:topology');
    const client = this.owner.lookup('service:client/http');
    const request = client.requestParams.bind(client);
    const expected = `GET /v1/internal/ui/service-topology/${id}?dc=${dc}&kind=${kind}`;
    const actual = adapter.requestForQueryRecord(request, {
      dc: dc,
      id: id,
      kind: kind,
    });
    assert.equal(`${actual.method} ${actual.url}`, expected);
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:topology');
    const client = this.owner.lookup('service:client/http');
    const request = client.url.bind(client);
    assert.throws(function() {
      adapter.requestForQueryRecord(request, {
        dc: dc,
      });
    });
  });
  test('requestForQueryRecord returns the correct body', function(assert) {
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
