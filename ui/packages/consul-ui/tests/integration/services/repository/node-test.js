import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
import { get } from '@ember/object';
const NAME = 'node';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});

const dc = 'dc-1';
const id = 'token-name';
const now = new Date().getTime();
const nspace = 'default';
const partition = 'default';
test('findByDatacenter returns the correct data for list endpoint', function(assert) {
  get(this.subject(), 'store').serializerFor(NAME).timestamp = function() {
    return now;
  };
  return repo(
    'Node',
    'findAllByDatacenter',
    this.subject(),
    function retrieveStub(stub) {
      return stub(
        `/v1/internal/ui/nodes?dc=${dc}${
          typeof partition !== 'undefined' ? `&partition=${partition}` : ``
        }`,
        {
          CONSUL_NODE_COUNT: '100',
        }
      );
    },
    function performTest(service) {
      return service.findAllByDatacenter({ dc, partition });
    },
    function performAssertion(actual, expected) {
      actual.forEach(item => {
        assert.equal(item.uid, `["${partition}","${nspace}","${dc}","${item.ID}"]`);
        assert.equal(item.Datacenter, dc);
      });
    }
  );
});
test('findBySlug returns the correct data for item endpoint', function(assert) {
  return repo(
    'Node',
    'findBySlug',
    this.subject(),
    function(stub) {
      return stub(
        `/v1/internal/ui/node/${id}?dc=${dc}${
          typeof partition !== 'undefined' ? `&partition=${partition}` : ``
        }`
      );
    },
    function(service) {
      return service.findBySlug({ id, dc, partition });
    },
    function(actual, expected) {
      assert.equal(actual.uid, `["${partition}","${nspace}","${dc}","${actual.ID}"]`);
      assert.equal(actual.Datacenter, dc);
    }
  );
});
