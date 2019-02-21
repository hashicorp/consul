import Controller from '@ember/controller';
import { set } from '@ember/object';

export default Controller.extend({
  setProperties: function() {
    this._super(...arguments);
    // This method is called immediately after `Route::setupController`, and done here rather than there
    // as this is a variable used purely for view level things, if the view was different we might not
    // need this variable
    set(this, 'selectedTab', 'service-checks');
  },
  actions: {
    change: function(e) {
      set(this, 'selectedTab', e.target.value);
    },
  },
});
