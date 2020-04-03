import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';
import WithEventSource, { listen } from 'consul-ui/mixins/with-event-source';
export default Controller.extend(WithEventSource, {
  dom: service('dom'),
  notify: service('flashMessages'),
  setProperties: function(model) {
    this._super(...arguments);
    // This method is called immediately after `Route::setupController`, and done here rather than there
    // as this is a variable used purely for view level things, if the view was different we might not
    // need the selectedTab variable
    const prev = get(this, 'history.firstObject.key') || '';
    if (prev.indexOf('dc.intentions.') !== -1 || prev.indexOf('dc.services.show') !== -1) {
      set(this, 'selectedTab', 'intentions');
    } else {
      set(this, 'selectedTab', 'instances');
    }
  },
  item: listen('item').catch(function(e) {
    if (e.target.readyState === 1) {
      // OPEN
      if (get(e, 'error.errors.firstObject.status') === '404') {
        this.notify.add({
          destroyOnClick: false,
          sticky: true,
          type: 'warning',
          action: 'update',
        });
      }
    }
  }),
});
