import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
import { get } from '@ember/object';
const NAME = 'coordinate';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});

const dc = 'dc-1';
const nspace = 'default';
const partition = 'default';
const now = new Date().getTime();
test('findAllByDatacenter returns the correct data for list endpoint', function(assert) {
  get(this.subject(), 'store').serializerFor(NAME).timestamp = function() {
    return now;
  };
  return repo(
    'Coordinate',
    'findAllByDatacenter',
    this.subject(),
    function retrieveStub(stub) {
      return stub(
        `/v1/coordinate/nodes?dc=${dc}${
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
      assert.deepEqual(
        actual,
        expected(function(payload) {
          return payload.map(item =>
            Object.assign({}, item, {
              SyncTime: now,
              Datacenter: dc,
              Partition: partition,
              // TODO: nspace isn't required here, once we've
              // refactored out our Serializer this can go
              uid: `["${partition}","${nspace}","${dc}","${item.Node}"]`,
            })
          );
        })
      );
    }
  );
});
test('findAllByNode calls findAllByDatacenter with the correct arguments', function(assert) {
  assert.expect(3);
  const datacenter = 'dc-1';
  const conf = {
    cursor: 1,
  };
  const service = this.subject();
  service.findAllByDatacenter = function(params, configuration) {
    assert.equal(arguments.length, 2, 'Expected to be called with the correct number of arguments');
    assert.equal(params.dc, datacenter);
    assert.deepEqual(configuration, conf);
    return Promise.resolve([]);
  };
  return service.findAllByNode({ node: 'node-name', dc: datacenter }, conf);
});
