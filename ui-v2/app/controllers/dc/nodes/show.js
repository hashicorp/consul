import Controller from '@ember/controller';
import { get, set, computed } from '@ember/object';
import { getOwner } from '@ember/application';
import WithSearching from 'consul-ui/mixins/with-searching';
import qsaFactory from 'consul-ui/utils/dom/qsa-factory';
import getComponentFactory from 'consul-ui/utils/get-component-factory';

const $$ = qsaFactory();
export default Controller.extend(WithSearching, {
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  init: function() {
    this.searchParams = {
      nodeservice: 's',
    };
    this._super(...arguments);
  },
  searchable: computed('items', function() {
    return get(this, 'searchables.nodeservice')
      .add(get(this, 'items'))
      .search(get(this, this.searchParams.nodeservice));
  }),
  setProperties: function() {
    this._super(...arguments);
    // the default selected tab depends on whether you have any healthchecks or not
    // so check the length here.
    // This method is called immediately after `Route::setupController`, and done here rather than there
    // as this is a variable used purely for view level things, if the view was different we might not
    // need this variable
    set(this, 'selectedTab', get(this.item, 'Checks.length') > 0 ? 'health-checks' : 'services');
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
