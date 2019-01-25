import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
const NAME = 'intention';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  integration: true,
});

const dc = 'dc-1';
const id = 'token-name';
test('findAllByDatacenter returns the correct data for list endpoint', function(assert) {
  return repo(
    'Intention',
    'findAllByDatacenter',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/connect/intentions?dc=${dc}`, {
        CONSUL_INTENTION_COUNT: '100',
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
              CreatedAt: new Date(item.CreatedAt),
              UpdatedAt: new Date(item.UpdatedAt),
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
    'Intention',
    'findBySlug',
    this.subject(),
    function(stub) {
      return stub(`/v1/connect/intentions/${id}?dc=${dc}`);
    },
    function(service) {
      return service.findBySlug(id, dc);
    },
    function(actual, expected) {
      assert.deepEqual(
        actual,
        expected(function(payload) {
          const item = payload;
          return Object.assign({}, item, {
            CreatedAt: new Date(item.CreatedAt),
            UpdatedAt: new Date(item.UpdatedAt),
            Datacenter: dc,
            uid: `["${dc}","${item.ID}"]`,
          });
        })
      );
    }
  );
});
