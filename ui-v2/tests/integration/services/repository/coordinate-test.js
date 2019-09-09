import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
import { get } from '@ember/object';
const NAME = 'coordinate';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});

const dc = 'dc-1';
const now = new Date().getTime();
test('findAllByDatacenter returns the correct data for list endpoint', function(assert) {
  get(this.subject(), 'store').serializerFor(NAME).timestamp = function() {
    return now;
  };
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
              SyncTime: now,
              Datacenter: dc,
              uid: `["${dc}","${item.Node}"]`,
            })
          );
        })
      );
    }
  );
});
