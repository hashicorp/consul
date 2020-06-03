import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { env } from '../../../env';
const shouldHaveNspace = function(nspace) {
  return typeof nspace !== 'undefined' && env('CONSUL_NSPACES_ENABLED');
};
module('Integration | Adapter | service', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const id = 'service-name';
  const undefinedNspace = 'default';
  [undefinedNspace, 'team-1', undefined].forEach(nspace => {
    test(`requestForQuery returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:service');
      const client = this.owner.lookup('service:client/http');
      const expected = `GET /v1/internal/ui/services?dc=${dc}`;
      let actual = adapter.requestForQuery(client.url, {
        dc: dc,
        ns: nspace,
      });
      actual = actual.split('\n');
      assert.equal(actual.shift().trim(), expected);
      actual = actual.join('\n').trim();
      assert.equal(actual, `${shouldHaveNspace(nspace) ? `ns=${nspace}` : ``}`);
    });
    test(`requestForQuery returns the correct url/method when called with gateway when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:service');
      const client = this.owner.lookup('service:client/http');
      const gateway = 'gateway';
      const expected = `GET /v1/internal/ui/gateway-services-nodes/${gateway}?dc=${dc}`;
      let actual = adapter.requestForQuery(client.url, {
        dc: dc,
        ns: nspace,
        gateway: gateway,
      });
      actual = actual.split('\n');
      assert.equal(actual.shift().trim(), expected);
      actual = actual.join('\n').trim();
      assert.equal(actual, `${shouldHaveNspace(nspace) ? `ns=${nspace}` : ``}`);
    });
    test(`requestForQueryRecord returns the correct url/method when nspace is ${nspace}`, function(assert) {
      const adapter = this.owner.lookup('adapter:service');
      const client = this.owner.lookup('service:client/http');
      const expected = `GET /v1/health/service/${id}?dc=${dc}`;
      let actual = adapter.requestForQueryRecord(client.url, {
        dc: dc,
        id: id,
        ns: nspace,
      });
      actual = actual.split('\n');
      assert.equal(actual.shift().trim(), expected);
      actual = actual.join('\n').trim();
      assert.equal(actual, `${shouldHaveNspace(nspace) ? `ns=${nspace}` : ``}`);
    });
  });
  test("requestForQueryRecord throws if you don't specify an id", function(assert) {
    const adapter = this.owner.lookup('adapter:service');
    const client = this.owner.lookup('service:client/http');
    assert.throws(function() {
      adapter.requestForQueryRecord(client.url, {
        dc: dc,
      });
    });
  });
});
