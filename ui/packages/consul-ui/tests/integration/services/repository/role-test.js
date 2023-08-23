import { moduleFor, test, skip } from 'ember-qunit';
import { get } from '@ember/object';
import repo from 'consul-ui/tests/helpers/repo';
import { createPolicies } from 'consul-ui/tests/helpers/normalizers';

const NAME = 'role';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
const now = new Date().getTime();
const dc = 'dc-1';
const id = 'role-name';
const undefinedNspace = 'default';
const undefinedPartition = 'default';
const partition = 'default';
[undefinedNspace, 'team-1', undefined].forEach(nspace => {
  test(`findByDatacenter returns the correct data for list endpoint when nspace is ${nspace}`, function(assert) {
    get(this.subject(), 'store').serializerFor(NAME).timestamp = function() {
      return now;
    };
    return repo(
      'Role',
      'findAllByDatacenter',
      this.subject(),
      function retrieveStub(stub) {
        return stub(
          `/v1/acl/roles?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
            typeof partition !== 'undefined' ? `&partition=${partition}` : ``
          }`,
          {
            CONSUL_ROLE_COUNT: '100',
          }
        );
      },
      function performTest(service) {
        return service.findAllByDatacenter({
          dc: dc,
          nspace: nspace || undefinedNspace,
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
                Policies: createPolicies(item),
              })
            );
          })
        );
      }
    );
  });
  // FIXME: For some reason this tries to initialize the metrics service?
  skip(`findBySlug returns the correct data for item endpoint when the nspace is ${nspace}`, function(assert) {
    return repo(
      'Role',
      'findBySlug',
      this.subject(),
      function retrieveStub(stub) {
        return stub(
          `/v1/acl/role/${id}?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
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
        assert.deepEqual(
          actual,
          expected(function(payload) {
            const item = payload;
            return Object.assign({}, item, {
              Datacenter: dc,
              Namespace: item.Namespace || undefinedNspace,
              Partition: item.Partition || undefinedPartition,
              uid: `["${partition || undefinedPartition}","${item.Namespace ||
                undefinedNspace}","${dc}","${item.ID}"]`,
              Policies: createPolicies(item),
            });
          })
        );
      }
    );
  });
});
