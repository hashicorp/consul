import { moduleFor, test, skip } from 'ember-qunit';
import { get } from '@ember/object';
import repo from 'consul-ui/tests/helpers/repo';
const NAME = 'policy';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
skip('translate returns the correct data for the translate endpoint');
const now = new Date().getTime();
const dc = 'dc-1';
const id = 'policy-name';
const undefinedNspace = 'default';
const undefinedPartition = 'default';
const partition = 'default';
[undefinedNspace, 'team-1', undefined].forEach(nspace => {
  test(`findByDatacenter returns the correct data for list endpoint when nspace is ${nspace}`, function(assert) {
    get(this.subject(), 'store').serializerFor(NAME).timestamp = function() {
      return now;
    };
    return repo(
      'Policy',
      'findAllByDatacenter',
      this.subject(),
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
  test(`findBySlug returns the correct data for item endpoint when the nspace is ${nspace}`, function(assert) {
    return repo(
      'Policy',
      'findBySlug',
      this.subject(),
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
