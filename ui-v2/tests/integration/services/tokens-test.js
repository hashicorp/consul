import { moduleFor, test, skip } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
moduleFor('service:tokens', 'Integration | Service | tokens', {
  // Specify the other units that are required for this test.
  needs: ['service:store', 'model:token', 'adapter:token', 'serializer:token', 'service:settings'],
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
          return payload.map(item =>
            Object.assign({}, item, {
              Datacenter: dc,
              CreateTime: new Date(item.CreateTime),
              uid: `["${dc}","${item.AccessorID}"]`,
            })
          );
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
          });
        })
      );
    }
  );
});
skip('clone returns the correct data for the clone endpoint');
