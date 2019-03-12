import Controller from '@ember/controller';
import { get, set, computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import WithSearching from 'consul-ui/mixins/with-searching';
import WithEventSource, { listen } from 'consul-ui/mixins/with-event-source';
export default Controller.extend(WithEventSource, WithSearching, {
  dom: service('dom'),
  notify: service('flashMessages'),
  items: alias('item.Nodes'),
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
      }
    }
  }),
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
