import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
import { skip } from 'qunit';

const NAME = 'auth-method';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
const dc = 'dc-1';
const id = 'auth-method-name';
const undefinedNspace = 'default';
const undefinedPartition = 'default';
const partition = 'default';
[undefinedNspace, 'team-1', undefined].forEach(nspace => {
  test(`findAllByDatacenter returns the correct data for list endpoint when nspace is ${nspace}`, function(assert) {
    return repo(
      'auth-method',
      'findAllByDatacenter',
      this.subject(),
      function retrieveStub(stub) {
        return stub(
          `/v1/acl/auth-methods?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
            typeof partition !== 'undefined' ? `&partition=${partition}` : ``
          }`,
          {
            CONSUL_AUTH_METHOD_COUNT: '3',
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
            return payload.map(function(item) {
              return Object.assign({}, item, {
                Datacenter: dc,
                Namespace: item.Namespace || undefinedNspace,
                Partition: item.Partition || undefinedPartition,
                uid: `["${item.Partition || undefinedPartition}","${item.Namespace ||
                  undefinedNspace}","${dc}","${item.Name}"]`,
              });
            });
          })
        );
      }
    );
  });
  skip(`findBySlug returns the correct data for item endpoint when the nspace is ${nspace}`, function(assert) {
    return repo(
      'AuthMethod',
      'findBySlug',
      this.subject(),
      function retrieveStub(stub) {
        return stub(
          `/v1/acl/auth-method/${id}?dc=${dc}${
            typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
          }`
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
              Namespace: item.Namespace || undefinedNspace,
              uid: `["${item.Namespace || undefinedNspace}","${dc}","${item.Name}"]`,
              meta: {
                cacheControl: undefined,
                cursor: undefined,
                dc: dc,
                nspace: item.Namespace || undefinedNspace,
              },
            });
          })
        );
      }
    );
  });
});
