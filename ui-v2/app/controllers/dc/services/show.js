import Controller from '@ember/controller';
import { get, set, computed } from '@ember/object';
import { inject as service } from '@ember/service';
import WithSearching from 'consul-ui/mixins/with-searching';
export default Controller.extend(WithSearching, {
  dom: service('dom'),
  init: function() {
    this.searchParams = {
      serviceInstance: 's',
    };
    this._super(...arguments);
  },
  setProperties: function() {
    this._super(...arguments);
    // This method is called immediately after `Route::setupController`, and done here rather than there
    // as this is a variable used purely for view level things, if the view was different we might not
    // need this variable
    set(this, 'selectedTab', 'instances');
  },
  searchable: computed('items', function() {
    return get(this, 'searchables.serviceInstance')
      .add(get(this, 'items'))
      .search(get(this, this.searchParams.serviceInstance));
  }),
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
  },
});
