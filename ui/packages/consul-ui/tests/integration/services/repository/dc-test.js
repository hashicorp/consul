import { moduleFor } from 'ember-qunit';
import { skip } from 'qunit';
import repo from 'consul-ui/tests/helpers/repo';
const NAME = 'dc';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
skip("findBySlug (doesn't interact with the API) but still needs an int test");
skip('findAll returns the correct data for list endpoint', function(assert) {
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
      actual.forEach((item, i) => {
        assert.equal(actual[i].Name, item.Name);
        assert.equal(item.Local, i === 0);
      });
    }
  );
});
