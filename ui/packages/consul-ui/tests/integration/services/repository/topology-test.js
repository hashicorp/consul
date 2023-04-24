import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';

moduleFor('service:repository/topology', 'Integration | Service | topology', {
  // Specify the other units that are required for this test.
  integration: true,
});
const dc = 'dc-1';
const id = 'slug';
const kind = '';
test('findBySlug returns the correct data for item endpoint', function(assert) {
  return repo(
    'Service',
    'findBySlug',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/internal/ui/service-topology/${id}?dc=${dc}&${kind}`, {
        CONSUL_DISCOVERY_CHAIN_COUNT: 1,
      });
    },
    function performTest(service) {
      return service.findBySlug({ id, kind, dc });
    },
    function performAssertion(actual, expected) {
      const result = expected(function(payload) {
        return Object.assign(
          {},
          {
            Datacenter: dc,
            uid: `["default","default","${dc}","${id}"]`,
            meta: {
              cacheControl: undefined,
              cursor: undefined,
            },
          },
          payload
        );
      });
      assert.equal(actual.Datacenter, result.Datacenter);
      assert.equal(actual.uid, result.uid);
    }
  );
});
