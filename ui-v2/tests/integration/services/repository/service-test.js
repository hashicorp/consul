import { moduleFor, test } from 'ember-qunit';
import { skip } from 'qunit';
import repo from 'consul-ui/tests/helpers/repo';
import { get } from '@ember/object';
const NAME = 'service';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
skip('findBySlug returns a sane tree');
const dc = 'dc-1';
const id = 'token-name';
const now = new Date().getTime();
const undefinedNspace = 'default';
[undefinedNspace, 'team-1', undefined].forEach(nspace => {
  test(`findByDatacenter returns the correct data for list endpoint when nspace is ${nspace}`, function(assert) {
    get(this.subject(), 'store').serializerFor(NAME).timestamp = function() {
      return now;
    };
    return repo(
      'Service',
      'findAllByDatacenter',
      this.subject(),
      function retrieveStub(stub) {
        return stub(
          `/v1/internal/ui/services?dc=${dc}${
            typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
          }`,
          {
            CONSUL_SERVICE_COUNT: '100',
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
  test(`findBySlug returns the correct data for item endpoint when the nspace is ${nspace}`, function(assert) {
    return repo(
      'Service',
      'findBySlug',
      this.subject(),
      function(stub) {
        return stub(
          `/v1/health/service/${id}?dc=${dc}${
            typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
          }`,
          {
            CONSUL_NODE_COUNT: 1,
          }
        );
      },
      function(service) {
        return service.findBySlug(id, dc, nspace || undefinedNspace);
      },
      function(actual, expected) {
        assert.deepEqual(
          actual,
          expected(function(payload) {
            // TODO: So this tree is all 'wrong', it's not having any major impact
            // this this tree needs revisting to something that makes more sense
            payload = Object.assign(
              {},
              { Nodes: payload },
              {
                Datacenter: dc,
                Namespace: payload[0].Service.Namespace || undefinedNspace,
                uid: `["${payload[0].Service.Namespace || undefinedNspace}","${dc}","${id}"]`,
              }
            );
            const nodes = payload.Nodes;
            const service = payload.Nodes[0];
            service.Nodes = nodes;
            service.Tags = [...new Set(payload.Nodes[0].Service.Tags)];
            service.Namespace = payload.Namespace;
            service.meta = {
              cursor: undefined,
              dc: dc,
              nspace: payload.Namespace,
            };

            return service;
          })
        );
      }
    );
  });
});
