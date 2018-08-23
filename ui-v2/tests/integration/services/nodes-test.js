import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
const NAME = 'node';
moduleFor(`service:${NAME}s`, `Integration | Service | ${NAME}s`, {
  // Specify the other units that are required for this test.
  needs: [
    'service:settings',
    'service:store',
    `adapter:${NAME}`,
    `serializer:${NAME}`,
    `model:${NAME}`,
    'service:coordinates',
    'adapter:coordinate',
    'serializer:coordinate',
    'model:coordinate',
  ],
});

const dc = 'dc-1';
const id = 'token-name';
test('findByDatacenter returns the correct data for list endpoint', function(assert) {
  return repo(
    'Node',
    'findAllByDatacenter',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/internal/ui/nodes?dc=${dc}`, {
        CONSUL_NODE_COUNT: '100',
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
              uid: `["${dc}","${item.ID}"]`,
            })
          );
        })
      );
    }
  );
});
test('findBySlug returns the correct data for item endpoint', function(assert) {
  return repo(
    'Node',
    'findBySlug',
    this.subject(),
    function(stub) {
      return stub(`/v1/internal/ui/node/${id}?dc=${dc}`);
    },
    function(service) {
      return service.findBySlug(id, dc);
    },
    function(actual, expected) {
      assert.deepEqual(
        actual,
        expected(function(payload) {
          const item = payload;
          return Object.assign({}, item, {
            Datacenter: dc,
            uid: `["${dc}","${item.ID}"]`,
          });
        })
      );
    }
  );
});
