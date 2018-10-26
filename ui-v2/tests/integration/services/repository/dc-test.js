import { moduleFor, test } from 'ember-qunit';
import { skip } from 'qunit';
import repo from 'consul-ui/tests/helpers/repo';
const NAME = 'dc';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
skip("findBySlug (doesn't interact with the API) but still needs an int test");
test('findAll returns the correct data for list endpoint', function(assert) {
  return repo(
    'Dc',
    'findAll',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/catalog/datacenters`, {
        CONSUL_DATACENTER_COUNT: '100',
      });
    },
    function performTest(service) {
      return service.findAll();
    },
    function performAssertion(actual, expected) {
      assert.deepEqual(
        actual,
        expected(function(payload) {
          return payload.map(item => ({ Name: item })).sort(function(a, b) {
            if (a.Name < b.Name) return -1;
            if (a.Name > b.Name) return 1;
            return 0;
          });
        })
      );
    }
  );
});
