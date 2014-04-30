// Add mixins
App.KvShowController = Ember.ObjectController.extend(Ember.Validations.Mixin);

//
// path: /
//
// The index is for choosing datacenters.
//
App.IndexController = Ember.Controller.extend({
});

//
// path: /:dc
//
App.DcController = Ember.Controller.extend({
  isDropdownVisible: false,

  checks: function() {
    var services = this.get('services');
    var checks = Ember.A()

    // loop over all of the services we have,
    // merge their checks into one.
    services.forEach(function(item) {
      checks = checks.concat(item.Checks)
    });

    // return the checks
    return checks
  }.property('checks'),

  checkMessage: function() {
    var checks = this.get('checks')

    // return the message for display
    if (this.get('hasFailingChecks') == true) {
      return checks.filterBy('Status', 'critical').get('length') + ' checks failing';
    } else {
      return checks.filterBy('Status', 'passing').get('length') + ' checks passing';
    }

  }.property('checkMessage'),

  hasFailingChecks: function() {
    var checks = this.get('checks')

    // Return a boolean if checks are failing.
    return (checks.filterBy('Status', 'critical').get('length') > 0);

  }.property('hasFailingChecks'),

  actions: {
    toggle: function(item){
      this.toggleProperty('isDropdownVisible');
    }
  }
})

//
// path: /:dc/services
//
// The index is for choosing services.
//
App.ServicesController = Ember.ArrayController.extend({
  needs: ['application']
});

//
// path: /:dc/services/:name
//
// An individual service.
//
App.ServicesShowController = Ember.Controller.extend({
  needs: ['services']
});

App.KvShowController.reopen({
  isLoading: false,

  actions: {
    createKey: function() {
      this.set('isLoading', true);

      var newKey = this.get('newKey');
      var topModel = this.get('topModel');

      // If we don't have a previous model to base
      // see our parent, or we're not at the root level,
      // strip the leading slash.
      if (!topModel || topModel.get('parentKey') != "/") {
        newKey.set('key', (topModel.get('parentKey') + newKey.get('key')));
      }

      // Persist newKey
      //

      Ember.run.later(this, function() {
        this.set('isLoading', false)
      }, 500);

    }
  }
});

App.KvEditController = Ember.Controller.extend({
  isLoading: false,

  actions: {
    updateKey: function() {
      var key = this.get("model");
      this.set('isLoading', true);

      Ember.run.later(this, function() {
        this.set('isLoading', false)
      }, 1500);

    },

    deleteKey: function() {
      var key = this.get("model");
      this.set('isLoading', true);

      Ember.run.later(this, function() {
        this.set('isLoading', false)
      }, 1000);

    }
  }

});
