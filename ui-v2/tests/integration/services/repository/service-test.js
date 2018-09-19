import { moduleFor, test } from 'ember-qunit';
import { skip } from 'qunit';
import repo from 'consul-ui/tests/helpers/repo';
const NAME = 'service';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
const dc = 'dc-1';
const id = 'token-name';
test('findByDatacenter returns the correct data for list endpoint', function(assert) {
  return repo(
    'Service',
    'findAllByDatacenter',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/internal/ui/services?dc=${dc}`, {
        CONSUL_SERVICE_COUNT: '100',
      });
    },
    function performTest(service) {
      return service.findAllByDatacenter(dc);
    },
    function performAssertion(actual, expected) {
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
skip('findBySlug returns a sane tree');
test('findBySlug returns the correct data for item endpoint', function(assert) {
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
      assert.deepEqual(
        actual,
        expected(function(payload) {
          // TODO: So this tree is all 'wrong', it's not having any major impact
          // this this tree needs revisting to something that makes more sense
          payload = Object.assign(
            {},
            { Nodes: payload },
            {
              Datacenter: dc,
              uid: `["${dc}","${id}"]`,
            }
          );
          const nodes = payload.Nodes;
          const service = payload.Nodes[0];
          service.Nodes = nodes;
          service.Tags = payload.Nodes[0].Service.Tags;

          return service;
        })
      );
    }
  );
});
