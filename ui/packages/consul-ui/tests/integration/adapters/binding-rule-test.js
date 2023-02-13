import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import getNspaceRunner from 'consul-ui/tests/helpers/get-nspace-runner';

const nspaceRunner = getNspaceRunner('binding-rule');
module('Integration | Adapter | binding-rule', function (hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  test('requestForQuery returns the correct url/method', function (assert) {
    const adapter = this.owner.lookup('adapter:binding-rule');
    const client = this.owner.lookup('service:client/http');
    const request = client.requestParams.bind(client);
    const expected = `GET /v1/acl/binding-rules?dc=${dc}`;
    const actual = adapter.requestForQuery(request, {
      dc: dc,
    });
    assert.equal(`${actual.method} ${actual.url}`, expected);
  });
  test('requestForQuery returns the correct body', function (assert) {
    assert.expect(2);
    return nspaceRunner(
      (adapter, serializer, client) => {
        const request = client.body.bind(client);
        return adapter.requestForQuery(request, {
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
