import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
import { get } from '@ember/object';
const NAME = 'service';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
const dc = 'dc-1';
const now = new Date().getTime();
const undefinedNspace = 'default';
[undefinedNspace, 'team-1', undefined].forEach(nspace => {
  test(`findGatewayBySlug returns the correct data for list endpoint when nspace is ${nspace}`, function(assert) {
    get(this.subject(), 'store').serializerFor(NAME).timestamp = function() {
      return now;
    };
    const gateway = 'gateway';
    const conf = {
      cursor: 1,
    };
    return repo(
      'Service',
      'findGatewayBySlug',
      this.subject(),
      function retrieveStub(stub) {
        return stub(
          `/v1/internal/ui/gateway-services-nodes/${gateway}?dc=${dc}${
            typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
          }`,
          {
            CONSUL_SERVICE_COUNT: '100',
          }
        );
      },
      function performTest(service) {
        return service.findGatewayBySlug(gateway, dc, nspace || undefinedNspace, conf);
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
                uid: `["${item.Namespace || undefinedNspace}","${dc}","${item.Name}"]`,
              })
            );
          })
        );
      }
    );
  });
});
