import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
const NAME = 'kv';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
const dc = 'dc-1';
const id = 'key-name';
test('findAllBySlug returns the correct data for list endpoint', function(assert) {
  return repo(
    'Kv',
    'findAllBySlug',
    this.subject(),
    function retrieveTest(stub) {
      return stub(`/v1/kv/${id}?keys&dc=${dc}`, {
        CONSUL_KV_COUNT: '1',
      });
    },
    function performTest(service) {
      return service.findAllBySlug(id, dc);
    },
    function performAssertion(actual, expected) {
      assert.deepEqual(
        actual,
        expected(function(payload) {
          return payload.map(item => {
            return {
              Datacenter: dc,
              uid: `["${dc}","${item}"]`,
              Key: item,
            };
          });
        })
      );
    }
  );
});
test('findAllBySlug returns the correct data for item endpoint', function(assert) {
  return repo(
    'Kv',
    'findAllBySlug',
    this.subject(),
    function(stub) {
      return stub(`/v1/kv/${id}?dc=${dc}`);
    },
    function(service) {
      return service.findBySlug(id, dc);
    },
    function(actual, expected) {
      assert.deepEqual(
        actual,
        expected(function(payload) {
          const item = payload[0];
          return Object.assign({}, item, {
            Datacenter: dc,
            uid: `["${dc}","${item.Key}"]`,
          });
        })
      );
    }
  );
});
