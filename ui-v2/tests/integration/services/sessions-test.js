import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
const NAME = 'session';
moduleFor(`service:${NAME}`, `Integration | Service | ${NAME}s`, {
  // Specify the other units that are required for this test.
  needs: [
    'service:settings',
    'service:store',
    `adapter:${NAME}`,
    `serializer:${NAME}`,
    `model:${NAME}`,
  ],
});

const dc = 'dc-1';
const id = 'node-name';
test('findByNode returns the correct data for list endpoint', function(assert) {
  return repo(
    'Session',
    'findByNode',
    this.subject(),
    function retrieveStub(stub) {
      return stub(`/v1/session/node/${id}?dc=${dc}`, {
        CONSUL_SESSION_COUNT: '100',
      });
    },
    function performTest(service) {
      return service.findByNode(id, dc);
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
test('findByKey returns the correct data for item endpoint', function(assert) {
  return repo(
    'Session',
    'findByKey',
    this.subject(),
    function(stub) {
      return stub(`/v1/session/info/${id}?dc=${dc}`);
    },
    function(service) {
      return service.findByKey(id, dc);
    },
    function(actual, expected) {
      assert.deepEqual(
        actual,
        expected(function(payload) {
          const item = payload[0];
          return Object.assign({}, item, {
            Datacenter: dc,
            uid: `["${dc}","${item.ID}"]`,
          });
        })
      );
    }
  );
});
