import { moduleFor, test, skip } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
import { createPolicies } from 'consul-ui/tests/helpers/normalizers';

const NAME = 'token';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
skip('clone returns the correct data for the clone endpoint');
const dc = 'dc-1';
const id = 'token-id';
const undefinedNspace = 'default';
const undefinedPartition = 'default';
const partition = 'default';
[undefinedNspace, 'team-1', undefined].forEach(nspace => {
  test(`findByDatacenter returns the correct data for list endpoint when nspace is ${nspace}`, function(assert) {
    return repo(
      'Token',
      'findAllByDatacenter',
      this.subject(),
      function retrieveStub(stub) {
        return stub(
          `/v1/acl/tokens?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
            typeof partition !== 'undefined' ? `&partition=${partition}` : ``
          }`,
          {
            CONSUL_TOKEN_COUNT: '100',
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
            return payload.map(function(item) {
              return Object.assign({}, item, {
                Datacenter: dc,
                CreateTime: new Date(item.CreateTime),
                Namespace: item.Namespace || undefinedNspace,
                Partition: item.Partition || undefinedPartition,
                uid: `["${item.Partition || undefinedPartition}","${item.Namespace ||
                  undefinedNspace}","${dc}","${item.AccessorID}"]`,
                Policies: createPolicies(item),
              });
            });
          })
        );
      }
    );
  });
  test(`findBySlug returns the correct data for item endpoint when nspace is ${nspace}`, function(assert) {
    return repo(
      'Token',
      'findBySlug',
      this.subject(),
      function retrieveStub(stub) {
        return stub(
          `/v1/acl/token/${id}?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
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
        expected(function(item) {
          assert.equal(
            actual.uid,
            `["${partition || undefinedPartition}","${nspace || undefinedNspace}","${dc}","${
              item.AccessorID
            }"]`
          );
          assert.equal(actual.Datacenter, dc);
          assert.deepEqual(actual.Policies, createPolicies(item));
        });
      }
    );
  });
  test(`findByPolicy returns the correct data when nspace is ${nspace}`, function(assert) {
    const policy = 'policy-1';
    return repo(
      'Token',
      'findByPolicy',
      this.subject(),
      function retrieveStub(stub) {
        return stub(
          `/v1/acl/tokens?dc=${dc}&policy=${policy}${
            typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
          }${typeof partition !== 'undefined' ? `&partition=${partition}` : ``}`,
          {
            CONSUL_TOKEN_COUNT: '100',
          }
        );
      },
      function performTest(service) {
        return service.findByPolicy({
          id: policy,
          dc,
          ns: nspace || undefinedNspace,
          partition: partition || undefinedPartition,
        });
      },
      function performAssertion(actual, expected) {
        assert.deepEqual(
          actual,
          expected(function(payload) {
            return payload.map(function(item) {
              return Object.assign({}, item, {
                Datacenter: dc,
                CreateTime: new Date(item.CreateTime),
                Namespace: item.Namespace || undefinedNspace,
                Partition: item.Partition || undefinedPartition,
                uid: `["${item.Partition || undefinedPartition}","${item.Namespace ||
                  undefinedNspace}","${dc}","${item.AccessorID}"]`,
                Policies: createPolicies(item),
              });
            });
          })
        );
      }
    );
  });
  test(`findByRole returns the correct data when nspace is ${nspace}`, function(assert) {
    const role = 'role-1';
    return repo(
      'Token',
      'findByPolicy',
      this.subject(),
      function retrieveStub(stub) {
        return stub(
          `/v1/acl/tokens?dc=${dc}&role=${role}${
            typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
          }${typeof partition !== 'undefined' ? `&partition=${partition}` : ``}`,
          {
            CONSUL_TOKEN_COUNT: '100',
          }
        );
      },
      function performTest(service) {
        return service.findByRole({
          id: role,
          dc,
          ns: nspace || undefinedNspace,
          partition: partition || undefinedPartition,
        });
      },
      function performAssertion(actual, expected) {
        assert.deepEqual(
          actual,
          expected(function(payload) {
            return payload.map(function(item) {
              return Object.assign({}, item, {
                Datacenter: dc,
                CreateTime: new Date(item.CreateTime),
                Namespace: item.Namespace || undefinedNspace,
                Partition: item.Partition || undefinedPartition,
                uid: `["${item.Partition || undefinedPartition}","${item.Namespace ||
                  undefinedNspace}","${dc}","${item.AccessorID}"]`,
                Policies: createPolicies(item),
              });
            });
          })
        );
      }
    );
  });
});
