import { moduleFor, test } from 'ember-qunit';
import { skip } from 'qunit';
import repo from 'consul-ui/tests/helpers/repo';
moduleFor('service:services', 'Integration | Service | services', {
  // Specify the other units that are required for this test.
  needs: [
    'service:store',
    'model:service',
    'adapter:service',
    'serializer:service',
    'service:settings',
  ],
});
const dc = 'dc-1';
const id = 'token-name';
test('findByDatacenter returns the correct data for list endpoint', function(assert) {
  return repo(
    'Service',
    'findAllByDatacenter',
    this.subject(),
    function(stub) {
      return stub(`/v1/internal/ui/services?dc=${dc}`, {
        CONSUL_SERVICE_COUNT: '100',
      });
    },
    function(service) {
      return service.findAllByDatacenter(dc);
    },
    function(actual, expected) {
      assert.deepEqual(
        actual,
        expected(function(payload) {
          return payload.map(item =>
            Object.assign({}, item, {
              Datacenter: dc,
              uid: `["${dc}","${item.Name}"]`,
            })
          );
        })
      );
    }
  );
});
skip('findBySlug returns the correct data for item endpoint', function(assert) {
  return repo(
    'Service',
    'findBySlug',
    this.subject(),
    function(stub) {
      return stub(`/v1/health/service/${id}?dc=${dc}`, {
        CONSUL_NODE_COUNT: 1,
      });
    },
    function(service) {
      return service.findBySlug(id, dc);
    },
    function(actual, expected) {
      // console.log(actual);
      assert.deepEqual(
        actual,
        expected(function(payload) {
          payload = { Nodes: payload };
          const nodes = payload.Nodes;
          const service = payload.Nodes[0];
          // console.log(service);
          service.Nodes = JSON.parse(JSON.stringify(nodes));
          // service.Tags = payload.Nodes[0].Service.Tags;

          return Object.assign({}, service, {
            Datacenter: dc,
            // uid: `["${dc}","${id}"]`,
          });
        })
      );
    }
  );
});
