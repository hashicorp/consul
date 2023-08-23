import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
import { get } from '@ember/object';
const NAME = 'session';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});

const dc = 'dc-1';
const id = 'node-name';
const now = new Date().getTime();
const undefinedNspace = 'default';
const undefinedPartition = 'default';
const partition = 'default';
[undefinedNspace, 'team-1', undefined].forEach(nspace => {
  test(`findByNode returns the correct data for list endpoint when the nspace is ${nspace}`, function(assert) {
    get(this.subject(), 'store').serializerFor(NAME).timestamp = function() {
      return now;
    };
    return repo(
      'Session',
      'findByNode',
      this.subject(),
      function retrieveStub(stub) {
        return stub(
          `/v1/session/node/${id}?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
            typeof partition !== 'undefined' ? `&partition=${partition}` : ``
          }`,
          {
            CONSUL_SESSION_COUNT: '100',
          }
        );
      },
      function performTest(service) {
        return service.findByNode({
          id,
          dc,
          ns: nspace || undefinedNspace,
          partition: partition || undefinedPartition,
        });
      },
      function performAssertion(actual, expected) {
        assert.deepEqual(
          actual,
          expected(function(payload) {
            return payload.map(item =>
              Object.assign({}, item, {
                SyncTime: now,
                Datacenter: dc,
                Namespace: item.Namespace || undefinedNspace,
                Partition: item.Partition || undefinedPartition,
                uid: `["${item.Partition || undefinedPartition}","${item.Namespace ||
                  undefinedNspace}","${dc}","${item.ID}"]`,
              })
            );
          })
        );
      }
    );
  });
  test(`findByKey returns the correct data for item endpoint when the nspace is ${nspace}`, function(assert) {
    return repo(
      'Session',
      'findByKey',
      this.subject(),
      function(stub) {
        return stub(
          `/v1/session/info/${id}?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
            typeof partition !== 'undefined' ? `&partition=${partition}` : ``
          }`
        );
      },
      function(service) {
        return service.findByKey({
          id,
          dc,
          ns: nspace || undefinedNspace,
          partition: partition || undefinedPartition,
        });
      },
      function(actual, expected) {
        assert.deepEqual(
          actual,
          expected(function(payload) {
            const item = payload[0];
            return Object.assign({}, item, {
              Datacenter: dc,
              Namespace: item.Namespace || undefinedNspace,
              Partition: item.Partition || undefinedPartition,
              uid: `["${item.Partition || undefinedPartition}","${item.Namespace ||
                undefinedNspace}","${dc}","${item.ID}"]`,
            });
          })
        );
      }
    );
  });
});
