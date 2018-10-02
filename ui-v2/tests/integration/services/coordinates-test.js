import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
const NAME = 'coordinate';
moduleFor(`service:${NAME}s`, `Integration | Service | ${NAME}s`, {
  // Specify the other units that are required for this test.
  needs: [
    'service:settings',
    'service:store',
    `adapter:${NAME}`,
    `serializer:${NAME}`,
    `model:${NAME}`,
  ],
});

const dc = 'dc-1';
test('findAllByDatacenter returns the correct data for list endpoint', function(assert) {
  return repo(
    'Coordinate',
    'findAllByDatacenter',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/coordinate/nodes?dc=${dc}`, {
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
              uid: `["${dc}","${item.Node}"]`,
            })
          );
        })
      );
    }
  );
});
