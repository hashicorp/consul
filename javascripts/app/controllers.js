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
    var services = this.get('nodes');
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
      var controller = this;

      // If we don't have a previous model to base
      // see our parent, or we're not at the root level,
      // strip the leading slash.
      if (!topModel || topModel.get('parentKey') != "/") {
        newKey.set('Key', (topModel.get('parentKey') + newKey.get('Key')));
      }

      Ember.$.ajax({
          url: "/v1/kv/" + newKey.get('Key'),
          type: 'PUT',
          data: newKey.get('Value')
      }).then(function(response) {
        controller.set('isLoading', false)
        controller.transitionToRoute('kv.edit', newKey.get('urlSafeKey'));
        controller.get('keys').reload()
      }).fail(function(response) {
        controller.set('errorMessage', 'Received error while processing: ' + response.statusText)
      });

    }
  }
});

App.KvEditController = Ember.Controller.extend({
  isLoading: false,

  actions: {
    updateKey: function() {
      this.set('isLoading', true);

      var key = this.get("model");
      var controller = this;

      Ember.$.ajax({
          url: "/v1/kv/" + key.get('Key'),
          type: 'PUT',
          data: key.get('valueDecoded')
      }).then(function(response) {
        controller.set('isLoading', false)
      }).fail(function(response) {
        controller.set('errorMessage', 'Received error while processing: ' + response.statusText)
      })
    },

    deleteKey: function() {
      this.set('isLoading', true);

      var key = this.get("model");
      var controller = this;
      var parent = key.get('urlSafeParentKey');

      Ember.$.ajax({
          url: "/v1/kv/" + key.get('Key'),
          type: 'DELETE'
      }).then(function(response) {
        controller.set('isLoading', false);
        controller.transitionToRoute('kv.show', parent);
      }).fail(function(response) {
        controller.set('errorMessage', 'Received error while processing: ' + response.statusText)
      })

    }
  }

});
