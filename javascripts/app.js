window.App = Ember.Application.create({
  rootElement: "#app",
  LOG_TRANSITIONS: true,
  currentPath: ''
});

App.Router.map(function() {
  this.route("index", { path: "/" });
  this.route("services", { path: "/:dc/services" });
  this.route("nodes", { path: "/:dc/nodes" });
  this.route("node", { path: "/:dc/nodes/:name" });
  this.route("kv", { path: "/:dc/kv" });
});

App.ApplicationController = Ember.Controller.extend({
  setDc: function(dc) {
    localStorage.setItem("current_dc", dc);
  },

  getDc: function() {
    return localStorage.getItem("current_dc");
  }.property("getDc")
});

App.ServicesController = Ember.Controller.extend({
  needs: ['application']
})

// Superclass to be used by all of the main routes below
App.BaseRoute = Ember.Route.extend({
  activate: function() {
    var controller = this.controllerFor('application');
    if (controller.getDc === null) {
      this.transitionTo('index');
    };
  }
});

// Does not extend baseroute due to it not needing
// to check for an active DC
App.IndexRoute = Ember.Route.extend({
  model: function() {
    return window.fixtures.dcs;
  },
  afterModel: function(dcs, transition) {
    if (dcs.get('length') === 1) {
      this.get('controllers.application').setDc(dcs[0])
      this.transitionTo('services', dcs[0]);
    }
  }
});

App.IndexController = Ember.Controller.extend({
  needs: ['application'],

  actions: {
    selectDc: function(dc) {
      this.get('controllers.application').setDc(dc)
      this.transitionToRoute('services', dc)
    }
  }
});

App.Service = Ember.Object.extend({
  failingChecks: function() {
    return this.get('Checks').filterBy('Status', 'critical').get('length');
  }.property('failingChecks'),

  passingChecks: function() {
    return this.get('Checks').filterBy('Status', 'passing').get('length');
  }.property('passingChecks'),

  checkMessage: function() {
    if (this.get('hasFailingChecks') === false) {
      return this.get('passingChecks') + ' passing';
    } else {
      return this.get('failingChecks') + ' failing';
    }
  }.property('checkMessage'),

  hasFailingChecks: function() {
    return (this.get('failingChecks') > 0);
  }.property('hasFailingChecks')
});

App.ServicesRoute = App.BaseRoute.extend({
  model: function() {
    return [App.Service.create(window.fixtures.services[0]), App.Service.create(window.fixtures.services[1])];
  }
});


App.ServicesView = Ember.View.extend({
    layoutName: 'default_layout'
})

