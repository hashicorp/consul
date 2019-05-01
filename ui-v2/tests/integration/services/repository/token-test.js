import { moduleFor, test, skip } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
import { createPolicies } from 'consul-ui/tests/helpers/normalizers';

const NAME = 'token';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
const dc = 'dc-1';
const id = 'token-id';
test('findByDatacenter returns the correct data for list endpoint', function(assert) {
  return repo(
    'Token',
    'findAllByDatacenter',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/acl/tokens?dc=${dc}`, {
        CONSUL_TOKEN_COUNT: '100',
      });
    },
    function performTest(service) {
      return service.findAllByDatacenter(dc);
    },
    function performAssertion(actual, expected) {
      assert.deepEqual(
        actual,
        expected(function(payload) {
          return payload.map(function(item) {
            return Object.assign({}, item, {
              Datacenter: dc,
              CreateTime: new Date(item.CreateTime),
              uid: `["${dc}","${item.AccessorID}"]`,
              Policies: createPolicies(item),
            });
          });
        })
      );
    }
  );
});
test('findBySlug returns the correct data for item endpoint', function(assert) {
  return repo(
    'Token',
    'findBySlug',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/acl/token/${id}?dc=${dc}`);
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
            CreateTime: new Date(item.CreateTime),
            uid: `["${dc}","${item.AccessorID}"]`,
            Policies: createPolicies(item),
          });
        })
      );
    }
  );
});
skip('clone returns the correct data for the clone endpoint');
