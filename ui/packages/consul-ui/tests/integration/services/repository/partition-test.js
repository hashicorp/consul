import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';

moduleFor('service:repository/partition', 'Integration | Repository | partition', {
  // Specify the other units that are required for this test.
  integration: true,
});
const dc = 'dc-1';
const id = 'slug';
const now = new Date().getTime();
test('findByDatacenter returns the correct data for list endpoint', function(assert) {
  this.subject().store.serializerFor('partition').timestamp = function() {
    return now;
  };
  return repo(
    'Service',
    'findAllByDatacenter',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/partition?dc=${dc}`, {
        CONSUL_PARTITION_COUNT: '100',
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
              SyncTime: now,
              Datacenter: dc,
              uid: `["${dc}","${item.Name}"]`,
            })
          );
        })
      );
    }
  );
});
test('findBySlug returns the correct data for item endpoint', function(assert) {
  return repo(
    'Service',
    'findBySlug',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/partition/${id}?dc=${dc}`, {
        CONSUL_PARTITION_COUNT: 1,
      });
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
              uid: `["${dc}","${id}"]`,
              meta: {
                cursor: undefined,
              },
            },
            payload
          );
        })
      );
    }
  );
});
