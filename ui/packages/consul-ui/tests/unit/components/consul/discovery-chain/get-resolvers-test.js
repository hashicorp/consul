import { getResolvers } from 'consul-ui/components/consul/discovery-chain/utils';
import { module, test } from 'qunit';
import { get } from 'consul-ui/tests/helpers/api';

const dc = 'dc-1';
const nspace = 'default';
const partition = 'default';
const request = {
  url: `/v1/discovery-chain/service-name?dc=${dc}`,
};
module('Unit | Component | consul/discovery-chain/get-resolvers', function() {
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
      const actual = getResolvers(dc, partition, nspace, Chain.Targets, Chain.Nodes);
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
      const actual = getResolvers(dc, partition, nspace, Chain.Targets, Chain.Nodes);
      const childId = Object.keys(Chain.Targets)[1];
      const target = Chain.Targets[`${childId}`];
      const firstChild = actual[0].Children[0];
      assert.equal(firstChild.Redirect, 'Datacenter');
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
          const actual = getResolvers(dc, partition, nspace, Chain.Targets, Chain.Nodes);
          const actualSubset = actual[0].Children[0];
          assert.equal(actualSubset.Subset, true);
          assert.equal(actualSubset.Failover.Type, failoverType);
        });
      })
    );
  });
  test('it assigns Failovers correctly', function(assert) {
    return Promise.all(
      ['Datacenter', 'Partition', 'Namespace'].map(function(failoverType, i) {
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
          const actual = getResolvers(dc, partition, nspace, Chain.Targets, Chain.Nodes);
          const node = Chain.Nodes[`resolver:${Object.keys(Chain.Targets)[0]}`];
          const expected = node.Resolver.Failover.Targets.map(item => item.split('.').reverse()[i]);
          assert.equal(actual[0].Failover.Type, failoverType);
          assert.deepEqual(actual[0].Failover.Targets, expected);
        });
      })
    );
  });
  test('it finds subsets with failovers correctly', function(assert) {
    return Promise.resolve({
      Chain: {
        ServiceName: 'service-name',
        Namespace: 'default',
        Datacenter: 'dc-1',
        Protocol: 'http',
        StartNode: '',
        Nodes: {
          'resolver:v2.dc-failover.default.default.dc-1': {
            Type: 'resolver',
            Name: 'v2.dc-failover.default.default.dc-1',
            Resolver: {
              Target: 'v2.dc-failover.default.default.dc-1',
              Failover: {
                Targets: [
                  'v2.dc-failover.default.default.dc-5',
                  'v2.dc-failover.default.default.dc-6',
                ],
              },
            },
          },
        },
        Targets: {
          'v2.dc-failover.default.default.dc-1': {
            ID: 'v2.dc-failover.default.default.dc-1',
            Service: 'dc-failover',
            Namespace: 'default',
            Partition: 'default',
            Datacenter: 'dc-1',
            Subset: {
              Filter: '',
            },
          },
          'v2.dc-failover.default.default.dc-6': {
            ID: 'v2.dc-failover.default.default.dc-6',
            Service: 'dc-failover',
            Namespace: 'default',
            Partition: 'default',
            Datacenter: 'dc-6',
            Subset: {
              Filter: '',
            },
          },
        },
      },
    }).then(function({ Chain }) {
      const actual = getResolvers(dc, partition, nspace, Chain.Targets, Chain.Nodes);
      const expected = {
        ID: 'dc-failover.default.default.dc-1',
        Name: 'dc-failover',
        Children: [
          {
            Subset: true,
            ID: 'v2.dc-failover.default.default.dc-1',
            Name: 'v2',
            Failover: {
              Type: 'Datacenter',
              Targets: ['dc-5', 'dc-6'],
            },
          },
        ],
      };
      assert.deepEqual(actual[0], expected);
    });
  });
  test('it finds services with failovers correctly', function(assert) {
    return Promise.resolve({
      Chain: {
        ServiceName: 'service-name',
        Namespace: 'default',
        Datacenter: 'dc-1',
        Protocol: 'http',
        StartNode: '',
        Nodes: {
          'resolver:dc-failover.default.default.dc-1': {
            Type: 'resolver',
            Name: 'dc-failover.default.default.dc-1',
            Resolver: {
              Target: 'dc-failover.default.default.dc-1',
              Failover: {
                Targets: ['dc-failover.default.default.dc-5', 'dc-failover.default.default.dc-6'],
              },
            },
          },
        },
        Targets: {
          'dc-failover.default.default.dc-1': {
            ID: 'dc-failover.default.default.dc-1',
            Service: 'dc-failover',
            Namespace: 'default',
            Partition: 'default',
            Datacenter: 'dc-1',
          },
        },
      },
    }).then(function({ Chain }) {
      const actual = getResolvers(dc, partition, nspace, Chain.Targets, Chain.Nodes);
      const expected = {
        ID: 'dc-failover.default.default.dc-1',
        Name: 'dc-failover',
        Children: [],
        Failover: {
          Type: 'Datacenter',
          Targets: ['dc-5', 'dc-6'],
        },
      };
      assert.deepEqual(actual[0], expected);
    });
  });
  test('it finds services with redirects with failovers correctly', function(assert) {
    return Promise.resolve({
      Chain: {
        ServiceName: 'service-name',
        Namespace: 'default',
        Partition: 'default',
        Datacenter: 'dc-1',
        Protocol: 'http',
        StartNode: '',
        Nodes: {
          'resolver:dc-failover.default.default.redirect-dc-1': {
            Type: 'resolver',
            Name: 'dc-failover.default.default.redirect-dc-1',
            Resolver: {
              Target: 'dc-failover.default.default.redirect-dc-1',
              Failover: {
                Targets: [
                  'dc-failover.default.default.redirect-dc-5',
                  'dc-failover.default.default.redirect-dc-6',
                ],
              },
            },
          },
        },
        Targets: {
          'dc-failover.default.default.redirect-dc-1': {
            ID: 'dc-failover.default.default.redirect-dc-1',
            Service: 'dc-failover',
            Namespace: 'default',
            Partition: 'default',
            Datacenter: 'redirect-dc-1',
          },
        },
      },
    }).then(function({ Chain }) {
      const actual = getResolvers(dc, partition, nspace, Chain.Targets, Chain.Nodes);
      // Both the parent and the child should have a Failover property
      // as in order for a redirect to have failovers it must redirect to a
      // service that already has failovers
      const expected = {
        ID: 'dc-failover.default.default.dc-1',
        Name: 'dc-failover',
        Failover: {
          Targets: ['redirect-dc-5', 'redirect-dc-6'],
          Type: 'Datacenter',
        },
        Children: [
          {
            Failover: {
              Targets: ['redirect-dc-5', 'redirect-dc-6'],
              Type: 'Datacenter',
            },
            ID: 'dc-failover.default.default.redirect-dc-1',
            Name: 'redirect-dc-1',
            Redirect: 'Datacenter',
          },
        ],
      };
      assert.deepEqual(actual[0], expected);
    });
  });
});
