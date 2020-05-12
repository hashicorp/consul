import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';

moduleFor('service:repository/gateway', 'Integration | Repository | gateway', {
  // Specify the other units that are required for this test.
  integration: true,
});
const dc = 'dc-1';
const id = 'slug';
const nspace = 'default';
test('findBySlug returns the correct data for item endpoint', function(assert) {
  return repo(
    'Gateway',
    'findBySlug',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/internal/ui/gateway-services-nodes/${id}`);
    },
    function performTest(service) {
      return service.findBySlug(id, dc);
    },
    function performAssertion(actual, expected) {
      assert.deepEqual(
        actual,
        expected(function(payload) {
          return Object.assign(
            {},
            {
              Datacenter: dc,
              Name: id,
              Namespace: nspace,
              uid: `["${nspace}","${dc}","${id}"]`,
            },
            {
              Services: payload,
            }
          );
        })
      );
    }
  );
});
