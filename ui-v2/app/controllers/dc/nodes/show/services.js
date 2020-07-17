import Controller from '@ember/controller';
import { get, computed } from '@ember/object';

export default Controller.extend({
  queryParams: {
    search: {
      as: 'filter',
      replace: true,
    },
  },
  checks: computed('item.Checks.[]', function() {
    const checks = {};
    get(this, 'item.Checks')
      .filter(item => {
        return item.ServiceID !== '';
      })
      .forEach(item => {
        if (typeof checks[item.ServiceID] === 'undefined') {
          checks[item.ServiceID] = [];
        }
        checks[item.ServiceID].push(item);
      });

    return checks;
  }),
});
