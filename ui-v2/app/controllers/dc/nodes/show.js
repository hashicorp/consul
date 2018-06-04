import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import { getOwner } from '@ember/application';
import WithFiltering from 'consul-ui/mixins/with-filtering';
import qsaFactory from 'consul-ui/utils/qsa-factory';
import getComponentFactory from 'consul-ui/utils/get-component-factory';

const $$ = qsaFactory();
export default Controller.extend(WithFiltering, {
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  setProperties: function() {
    this._super(...arguments);
    set(this, 'selectedTab', 'health-checks');
  },
  filter: function(item, { s = '' }) {
    const term = s.toLowerCase();
    return (
      get(item, 'Service')
        .toLowerCase()
        .indexOf(term) !== -1 ||
      get(item, 'Port')
        .toString()
        .toLowerCase()
        .indexOf(term) !== -1
    );
  },
  actions: {
    change: function(e) {
      set(this, 'selectedTab', e.target.value);
      const getComponent = getComponentFactory(getOwner(this));
      // Ensure tabular-collections sizing is recalculated
      // now it is visible in the DOM
      [...$$('.tab-section input[type="radio"]:checked + div table')].forEach(function(item) {
        const component = getComponent(item);
        if (component && typeof component.didAppear === 'function') {
          getComponent(item).didAppear();
        }
      });
    },
    sortChecksByImportance: function(a, b) {
      const statusA = get(a, 'Status');
      const statusB = get(b, 'Status');
      switch (statusA) {
        case 'passing':
          // a = passing
          // unless b is also passing then a is less important
          return statusB === 'passing' ? 0 : 1;
        case 'critical':
          // a = critical
          // unless b is also critical then a is more important
          return statusB === 'critical' ? 0 : -1;
        case 'warning':
          // a = warning
          switch (statusB) {
            // b is passing so a is more important
            case 'passing':
              return -1;
            // b is critical so a is less important
            case 'critical':
              return 1;
            // a and b are both warning, therefore equal
            default:
              return 0;
          }
      }
      return 0;
    },
  },
});
