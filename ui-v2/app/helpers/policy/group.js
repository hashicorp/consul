import { helper } from '@ember/component/helper';
import { get } from '@ember/object';

const MANAGEMENT_ID = '00000000-0000-0000-0000-000000000001';

export default helper(function policyGroup([items] /*, hash*/) {
  return items.reduce(
    function(prev, item) {
      let group;
      switch (true) {
        case get(item, 'ID') === MANAGEMENT_ID:
          group = 'management';
          break;
        case get(item, 'template') !== '':
          group = 'identities';
          break;
        default:
          group = 'policies';
          break;
      }
      prev[group].push(item);
      return prev;
    },
    {
      management: [],
      identities: [],
      policies: [],
    }
  );
});
