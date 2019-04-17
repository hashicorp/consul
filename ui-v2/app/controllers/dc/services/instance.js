import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';
import WithEventSource, { listen } from 'consul-ui/mixins/with-event-source';

export default Controller.extend(WithEventSource, {
  notify: service('flashMessages'),
  setProperties: function() {
    this._super(...arguments);
    // This method is called immediately after `Route::setupController`, and done here rather than there
    // as this is a variable used purely for view level things, if the view was different we might not
    // need this variable
    set(this, 'selectedTab', 'service-checks');
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
        const proxy = get(this, 'proxy');
        if (proxy) {
          proxy.close();
        }
      }
    }
  }),
  actions: {
    change: function(e) {
      set(this, 'selectedTab', e.target.value);
    },
  },
});
