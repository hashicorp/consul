import { module, skip, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';

module(`Integration | Service | policy`, function (hooks) {
  setupTest(hooks);
  skip('translate returns the correct data for the translate endpoint');
  const now = new Date().getTime();
  const dc = 'dc-1';
  const id = 'policy-name';
  const undefinedNspace = 'default';
  const undefinedPartition = 'default';
  const partition = 'default';
  [undefinedNspace, 'team-1', undefined].forEach((nspace) => {
    test(`findByDatacenter returns the correct data for list endpoint when nspace is ${nspace}`, function (assert) {
      const subject = this.owner.lookup('service:repository/policy');

      subject.store.serializerFor('policy').timestamp = function () {
        return now;
      };
      return repo(
        'Policy',
        'findAllByDatacenter',
        subject,
        function retrieveStub(stub) {
          return stub(
            `/v1/acl/policies?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
              typeof partition !== 'undefined' ? `&partition=${partition}` : ``
            }`,
            {
              CONSUL_POLICY_COUNT: '10',
            }
          );
        },
        function performTest(service) {
          return service.findAllByDatacenter({
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
    test(`findBySlug returns the correct data for item endpoint when the nspace is ${nspace}`, function (assert) {
      assert.expect(2);
      const subject = this.owner.lookup('service:repository/policy');
      return repo(
        'Policy',
        'findBySlug',
        subject,
        function retrieveStub(stub) {
          return stub(
            `/v1/acl/policy/${id}?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
              typeof partition !== 'undefined' ? `&partition=${partition}` : ``
            }`
          );
        },
        function performTest(service) {
          return service.findBySlug({
            id,
            dc,
            ns: nspace || undefinedNspace,
            partition: partition || undefinedPartition,
          });
        },
        function performAssertion(actual, expected) {
          assert.equal(
            actual.uid,
            `["${partition || undefinedPartition}","${nspace || undefinedNspace}","${dc}","${
              actual.ID
            }"]`
          );
          assert.equal(actual.Datacenter, dc);
        }
      );
    });
  });
});
