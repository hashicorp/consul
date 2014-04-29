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
