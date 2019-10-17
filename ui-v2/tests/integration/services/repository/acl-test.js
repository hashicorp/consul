import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
const NAME = 'acl';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
const dc = 'dc-1';
const nspace = 'default';
const id = 'token-name';
test('findByDatacenter returns the correct data for list endpoint', function(assert) {
  return repo(
    'Acl',
    'findAllByDatacenter',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/acl/list?dc=${dc}`, {
        CONSUL_ACL_COUNT: '100',
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
              // TODO: default isn't required here, once we've
              // refactored out our Serializer this can go
              uid: `["${nspace}","${dc}","${item.ID}"]`,
            })
          );
        })
      );
    }
  );
});
test('findBySlug returns the correct data for item endpoint', function(assert) {
  return repo(
    'Acl',
    'findBySlug',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/acl/info/${id}?dc=${dc}`);
    },
    function performTest(service) {
      return service.findBySlug(id, dc);
    },
    function performAssertion(actual, expected) {
      assert.deepEqual(
        actual,
        expected(function(payload) {
          const item = payload[0];
          return Object.assign({}, item, {
            Datacenter: dc,
            // TODO: default isn't required here, once we've
            // refactored out our Serializer this can go
            uid: `["${nspace}","${dc}","${item.ID}"]`,
          });
        })
      );
    }
  );
});
