import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
import {
  HEADERS_DEFAULT_ACL_POLICY as DEFAULT_ACL_POLICY,
} from 'consul-ui/utils/http/consul';
module('Integration | Serializer | dc', function(hooks) {
  setupTest(hooks);
  test('respondForQuery returns the correct data for list endpoint', function(assert) {
    const serializer = this.owner.lookup('serializer:dc');
    let env = this.owner.lookup('service:env');
    env = env.var.bind(env);
    const request = {
      url: `/v1/catalog/datacenters`,
    };
    return get(request.url).then(function(payload) {
      const ALLOW = 'allow';
      const expected = payload.map(item => (
        {
          Name: item,
          Datacenter: '',
          Local: item === env('CONSUL_DATACENTER_LOCAL'),
          Primary: item === env('CONSUL_DATACENTER_PRIMARY'),
          DefaultACLPolicy: ALLOW
        }
      ))
      const actual = serializer.respondForQuery(function(cb) {
        const headers = {
          [DEFAULT_ACL_POLICY]: ALLOW
        };
        return cb(headers, payload);
      }, {
        dc: '*',
      });
      actual.forEach((item, i) => {
        assert.equal(actual[i].Name, expected[i].Name);
        assert.equal(actual[i].Local, expected[i].Local);
        assert.equal(actual[i].Primary, expected[i].Primary);
        assert.equal(actual[i].DefaultACLPolicy, expected[i].DefaultACLPolicy);
      });
    });
  });
});
