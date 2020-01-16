import { getResolvers } from 'consul-ui/utils/components/discovery-chain/index';
import { module, test } from 'qunit';
import { get } from 'consul-ui/tests/helpers/api';

const dc = 'dc-1';
const nspace = 'default';
const request = {
  url: `/v1/discovery-chain/service-name?dc=${dc}`,
};
module('Unit | Utility | components/discovery-chain/get-resolvers', function() {
  test('it assigns Subsets correctly', function(assert) {
    return get(request.url, {
      headers: {
        cookie: {
          CONSUL_RESOLVER_COUNT: 1,
          CONSUL_SUBSET_COUNT: 1,
          CONSUL_REDIRECT_COUNT: 0,
          CONSUL_FAILOVER_COUNT: 0,
        },
      },
    }).then(function({ Chain }) {
      const actual = getResolvers(dc, nspace, Chain.Targets, Chain.Nodes);
      const childId = Object.keys(Chain.Targets)[1];
      const target = Chain.Targets[`${childId}`];
      const firstChild = actual[0].Children[0];
      assert.equal(firstChild.Subset, true);
      assert.equal(firstChild.ID, target.ID);
      assert.equal(firstChild.Name, target.ServiceSubset);
    });
  });
  test('it assigns Redirects correctly', function(assert) {
    return get(request.url, {
      headers: {
        cookie: {
          CONSUL_RESOLVER_COUNT: 1,
          CONSUL_REDIRECT_COUNT: 1,
          CONSUL_FAILOVER_COUNT: 0,
          CONSUL_SUBSET_COUNT: 0,
        },
      },
    }).then(function({ Chain }) {
      const actual = getResolvers(dc, nspace, Chain.Targets, Chain.Nodes);
      const childId = Object.keys(Chain.Targets)[1];
      const target = Chain.Targets[`${childId}`];
      const firstChild = actual[0].Children[0];
      assert.equal(firstChild.Redirect, true);
      assert.equal(firstChild.ID, target.ID);
    });
  });
  test('it assigns Failovers to Subsets correctly', function(assert) {
    return Promise.all(
      ['Datacenter', 'Namespace'].map(function(failoverType) {
        return get(request.url, {
          headers: {
            cookie: {
              CONSUL_RESOLVER_COUNT: 1,
              CONSUL_REDIRECT_COUNT: 0,
              CONSUL_SUBSET_COUNT: 1,
              CONSUL_FAILOVER_COUNT: 1,
              CONSUL_FAILOVER_TYPE: failoverType,
            },
          },
        }).then(function({ Chain }) {
          const actual = getResolvers(dc, nspace, Chain.Targets, Chain.Nodes);
          const actualSubset = actual[0].Children[0];
          assert.equal(actualSubset.Subset, true);
          assert.equal(actualSubset.Failover.Type, failoverType);
        });
      })
    );
  });
  test('it assigns Failovers correctly', function(assert) {
    return Promise.all(
      ['Datacenter', 'Namespace'].map(function(failoverType, i) {
        return get(request.url, {
          headers: {
            cookie: {
              CONSUL_RESOLVER_COUNT: 1,
              CONSUL_REDIRECT_COUNT: 0,
              CONSUL_SUBSET_COUNT: 0,
              CONSUL_FAILOVER_COUNT: 1,
              CONSUL_FAILOVER_TYPE: failoverType,
            },
          },
        }).then(function({ Chain }) {
          const actual = getResolvers(dc, nspace, Chain.Targets, Chain.Nodes);
          const node = Chain.Nodes[`resolver:${Object.keys(Chain.Targets)[0]}`];
          const expected = node.Resolver.Failover.Targets.map(item => item.split('.').reverse()[i]);
          assert.equal(actual[0].Failover.Type, failoverType);
          assert.deepEqual(actual[0].Failover.Targets, expected);
        });
      })
    );
  });
});
