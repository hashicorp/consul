import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';

module(`Integration | Service | session`, function (hooks) {
  setupTest(hooks);

  const dc = 'dc-1';
  const id = 'node-name';
  const now = new Date().getTime();
  const undefinedNspace = 'default';
  const undefinedPartition = 'default';
  const partition = 'default';
  [undefinedNspace, 'team-1', undefined].forEach((nspace) => {
    test(`findByNode returns the correct data for list endpoint when the nspace is ${nspace}`, function (assert) {
      const subject = this.owner.lookup('service:repository/session');

      subject.store.serializerFor('session').timestamp = function () {
        return now;
      };
      return repo(
        'Session',
        'findByNode',
        subject,
        function retrieveStub(stub) {
          return stub(
            `/v1/session/node/${id}?dc=${dc}${
              typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
            }${typeof partition !== 'undefined' ? `&partition=${partition}` : ``}`,
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
          assert.propContains(
            actual,
            expected(function (payload) {
              return payload.map((item) =>
                Object.assign({}, item, {
                  SyncTime: now,
                  Datacenter: dc,
                  Namespace: item.Namespace || undefinedNspace,
                  Partition: item.Partition || undefinedPartition,
                  uid: `["${item.Partition || undefinedPartition}","${
                    item.Namespace || undefinedNspace
                  }","${dc}","${item.ID}"]`,
                })
              );
            })
          );
        }
      );
    });
    test(`findByKey returns the correct data for item endpoint when the nspace is ${nspace}`, function (assert) {
      const subject = this.owner.lookup('service:repository/session');
      return repo(
        'Session',
        'findByKey',
        subject,
        function (stub) {
          return stub(
            `/v1/session/info/${id}?dc=${dc}${
              typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
            }${typeof partition !== 'undefined' ? `&partition=${partition}` : ``}`
          );
        },
        function (service) {
          return service.findByKey({
            id,
            dc,
            ns: nspace || undefinedNspace,
            partition: partition || undefinedPartition,
          });
        },
        function (actual, expected) {
          assert.propContains(
            actual,
            expected(function (payload) {
              const item = payload[0];
              return Object.assign({}, item, {
                Datacenter: dc,
                Namespace: item.Namespace || undefinedNspace,
                Partition: item.Partition || undefinedPartition,
                uid: `["${item.Partition || undefinedPartition}","${
                  item.Namespace || undefinedNspace
                }","${dc}","${item.ID}"]`,
              });
            })
          );
        }
      );
    });
  });
});
