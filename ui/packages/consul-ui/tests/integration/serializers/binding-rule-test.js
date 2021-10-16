import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { get } from 'consul-ui/tests/helpers/api';
module('Integration | Serializer | binding-rule', function(hooks) {
  setupTest(hooks);
  const dc = 'dc-1';
  const undefinedNspace = 'default';
  const undefinedPartition = 'default';
  const partition = 'default';
  [undefinedNspace, 'team-1', undefined].forEach(nspace => {
    test(`respondForQuery returns the correct data for list endpoint when nspace is ${nspace}`, function(assert) {
      const serializer = this.owner.lookup('serializer:binding-rule');
      const request = {
        url: `/v1/acl/binding-rules?dc=${dc}${
          typeof nspace !== 'undefined' ? `&ns=${nspace}` : ``
        }${typeof partition !== 'undefined' ? `&partition=${partition}` : ``}`,
      };
      return get(request.url).then(function(payload) {
        const expected = payload.map(item =>
          Object.assign({}, item, {
            Datacenter: dc,
            Namespace: item.Namespace || undefinedNspace,
            Partition: item.Partition || undefinedPartition,
            uid: `["${payload.Partition || undefinedPartition}","${item.Namespace ||
              undefinedNspace}","${dc}","${item.ID}"]`,
          })
        );
        const actual = serializer.respondForQuery(
          function(cb) {
            const headers = {};
            const body = payload;
            return cb(headers, body);
          },
          {
            dc: dc,
            ns: nspace,
            partition: partition,
          }
        );
        assert.deepEqual(actual, expected);
      });
    });
  });
});
