import Controller from '@ember/controller';
import { alias } from '@ember/object/computed';
import { get, computed } from '@ember/object';

export default Controller.extend({
  items: alias('item.Services'),
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
