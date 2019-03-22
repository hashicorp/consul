import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { get, set, computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import WithSearching from 'consul-ui/mixins/with-searching';
import WithEventSource, { listen } from 'consul-ui/mixins/with-event-source';

export default Controller.extend(WithEventSource, WithSearching, {
  dom: service('dom'),
  notify: service('flashMessages'),
  items: alias('item.Services'),
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
  item: listen('item').catch(function(e) {
    if (e.target.readyState === 1) {
      // OPEN
      if (get(e, 'error.errors.firstObject.status') === '404') {
        get(this, 'notify').add({
          destroyOnClick: false,
          sticky: true,
          type: 'warning',
          action: 'update',
        });
        get(this, 'tomography').close();
        get(this, 'sessions').close();
      }
    }
  }),
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
    set(this, 'selectedTab', get(this, 'item.Checks.length') > 0 ? 'health-checks' : 'services');
  },
  actions: {
    change: function(e) {
      set(this, 'selectedTab', e.target.value);
      // Ensure tabular-collections sizing is recalculated
      // now it is visible in the DOM
      get(this, 'dom')
        .components('.tab-section input[type="radio"]:checked + div table')
        .forEach(function(item) {
          if (typeof item.didAppear === 'function') {
            item.didAppear();
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
