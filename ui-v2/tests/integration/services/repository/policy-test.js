import { moduleFor, test, skip } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
const NAME = 'policy';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
const dc = 'dc-1';
const id = 'policy-name';
test('findByDatacenter returns the correct data for list endpoint', function(assert) {
  return repo(
    'Policy',
    'findAllByDatacenter',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/acl/policies?dc=${dc}`, {
        CONSUL_POLICY_COUNT: '100',
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
    'Policy',
    'findBySlug',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/acl/policy/${id}?dc=${dc}`);
    },
    function performTest(service) {
      return service.findBySlug(id, dc);
    },
    function performAssertion(actual, expected) {
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
skip('translate returns the correct data for the translate enpoint');
