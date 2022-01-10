import { moduleFor, test } from 'ember-qunit';
import repo from 'consul-ui/tests/helpers/repo';
import { env } from '../../../../env';
import { get } from '@ember/object';

const NAME = 'kv';
moduleFor(`service:repository/${NAME}`, `Integration | Service | ${NAME}`, {
  // Specify the other units that are required for this test.
  integration: true,
});
const dc = 'dc-1';
const id = 'key-name';
const now = new Date().getTime();
const undefinedNspace = 'default';
const undefinedPartition = 'default';
const partition = 'default';
[undefinedNspace, 'team-1', undefined].forEach(nspace => {
  test(`findAllBySlug returns the correct data for list endpoint when nspace is ${nspace}`, function(assert) {
    get(this.subject(), 'store').serializerFor(NAME).timestamp = function() {
      return now;
    };
    return repo(
      'Kv',
      'findAllBySlug',
      this.subject(),
      function retrieveTest(stub) {
        return stub(
          `/v1/kv/${id}?keys&dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
            typeof partition !== 'undefined' ? `&partition=${partition}` : ``
          }`,
          {
            CONSUL_KV_COUNT: '1',
          }
        );
      },
      function performTest(service) {
        return service.findAllBySlug({
          id,
          dc,
          ns: nspace || undefinedNspace,
          partition: partition || undefinedPartition,
        });
      },
      function performAssertion(actual, expected) {
        const expectedNspace = env('CONSUL_NSPACES_ENABLED')
          ? nspace || undefinedNspace
          : 'default';
        const expectedPartition = env('CONSUL_PARTITIONS_ENABLED')
          ? partition || undefinedPartition
          : 'default';
        actual.forEach(item => {
          assert.equal(item.uid, `["${expectedPartition}","${expectedNspace}","${dc}","${item.Key}"]`);
          assert.equal(item.Datacenter, dc);
        });
      }
    );
  });
  test(`findBySlug returns the correct data for item endpoint when nspace is ${nspace}`, function(assert) {
    return repo(
      'Kv',
      'findBySlug',
      this.subject(),
      function(stub) {
        return stub(
          `/v1/kv/${id}?dc=${dc}${typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``}${
            typeof partition !== 'undefined' ? `&partition=${partition}` : ``
          }`
        );
      },
      function(service) {
        return service.findBySlug({
          id,
          dc,
          ns: nspace || undefinedNspace,
          partition: partition || undefinedPartition,
        });
      },
      function(actual, expected) {
        expected(
          function(payload) {
            const item = payload[0];
            assert.equal(actual.uid, `["${item.Partition || undefinedPartition}","${item.Namespace ||
                      undefinedNspace}","${dc}","${item.Key}"]`);
            assert.equal(actual.Datacenter, dc);
          }
        );
      }
    );
  });
});
