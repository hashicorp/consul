import { moduleFor, test, skip } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
const NAME = 'policy';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
skip('translate returns the correct data for the translate endpoint');
const dc = 'dc-1';
const id = 'policy-name';
const undefinedNspace = 'default';
[undefinedNspace, 'team-1', undefined].forEach(nspace => {
  test(`findByDatacenter returns the correct data for list endpoint when nspace is ${nspace}`, function(assert) {
    return repo(
      'Policy',
      'findAllByDatacenter',
      this.subject(),
      function retrieveStub(stub) {
        return stub(
          `/v1/acl/policies?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}`,
          {
            CONSUL_POLICY_COUNT: '100',
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
            return payload.map(item =>
              Object.assign({}, item, {
                Datacenter: dc,
                Namespace: item.Namespace || undefinedNspace,
                uid: `["${item.Namespace || undefinedNspace}","${dc}","${item.ID}"]`,
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
          `/v1/acl/policy/${id}?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}`
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
              uid: `["${item.Namespace || undefinedNspace}","${dc}","${item.ID}"]`,
            });
          })
        );
      }
    );
  });
});
