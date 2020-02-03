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
[undefinedNspace, 'team-1', undefined].forEach(nspace => {
  test(`findByDatacenter returns the correct data for list endpoint when nspace is ${nspace}`, function(assert) {
    return repo(
      'Token',
      'findAllByDatacenter',
      this.subject(),
      function retrieveStub(stub) {
        return stub(
          `/v1/acl/tokens?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}`,
          {
            CONSUL_TOKEN_COUNT: '100',
          }
        );
      },
      function performTest(service) {
        return service.findAllByDatacenter(dc, nspace || undefinedNspace);
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
                uid: `["${item.Namespace || undefinedNspace}","${dc}","${item.AccessorID}"]`,
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
          `/v1/acl/token/${id}?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}`
        );
      },
      function performTest(service) {
        return service.findBySlug(id, dc, nspace || undefinedNspace);
      },
      function performAssertion(actual, expected) {
        assert.deepEqual(
          actual,
          expected(function(payload) {
            const item = payload;
            return Object.assign({}, item, {
              Datacenter: dc,
              CreateTime: new Date(item.CreateTime),
              Namespace: item.Namespace || undefinedNspace,
              uid: `["${item.Namespace || undefinedNspace}","${dc}","${item.AccessorID}"]`,
              Policies: createPolicies(item),
            });
          })
        );
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
          }`,
          {
            CONSUL_TOKEN_COUNT: '100',
          }
        );
      },
      function performTest(service) {
        return service.findByPolicy(policy, dc, nspace || undefinedNspace);
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
                uid: `["${item.Namespace || undefinedNspace}","${dc}","${item.AccessorID}"]`,
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
          }`,
          {
            CONSUL_TOKEN_COUNT: '100',
          }
        );
      },
      function performTest(service) {
        return service.findByRole(role, dc, nspace || undefinedNspace);
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
                uid: `["${item.Namespace || undefinedNspace}","${dc}","${item.AccessorID}"]`,
                Policies: createPolicies(item),
              });
            });
          })
        );
      }
    );
  });
});
