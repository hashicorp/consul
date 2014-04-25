//
// Superclass to be used by all of the main routes below. All routes
// but the IndexRoute share the need to have a datacenter set.
//
//
App.BaseRoute = Ember.Route.extend({
  //
  // When activating the base route, if we don't have a datacenter set,
  // transition the user to the index route to choose a datacenter.
  //
  activate: function() {
    var controller = this.controllerFor('application');
    if (controller.getDc === null) {
      this.transitionTo('index');
    };
  }
});

//
// The route for choosing datacenters, typically the first route loaded.
//
// Note: This *does not* extend from BaseRoute as that could cause
// and loop of transitions.
//
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

// The base DC route

App.DcRoute = App.BaseRoute.extend({
  //
  // Set the model on the route. We look up the specific service
  // by it's identifier passed via the route
  //
  model: function(params) {
    return params.dc;
  }
});


/// services

//
// Display all the services, allow to drill down into the specific services.
//
App.ServicesRoute = App.BaseRoute.extend({
  //
  // Set the services as the routes default model to be called in
  // the template as {{model}}
  //
  setupController: function(controller, model) {
      //
      // Since we have 2 column layout, we need to also display the
      // list of services on the left. Hence setting the attribute
      // {{services}} on the controller.
      //
      controller.set('services', [App.Service.create(window.fixtures.services[0]), App.Service.create(window.fixtures.services[1])]);
  }
});


//
// Display an individual service, as well as the global services in the left
// column.
//
App.ServicesShowRoute = App.BaseRoute.extend({
  //
  // Set the model on the route. We look up the specific service
  // by it's identifier passed via the route
  //
  model: function(params) {
    return [App.Node.create(window.fixtures.services_full[params.name][0]), App.Node.create(window.fixtures.services_full[params.name][1])];
  },

  setupController: function(controller, model) {
      controller.set('content', model);
      //
      // Since we have 2 column layout, we need to also display the
      // list of services on the left. Hence setting the attribute
      // {{services}} on the controller.
      //
      controller.set('services', [App.Service.create(window.fixtures.services[0]), App.Service.create(window.fixtures.services[1])]);
  }
});


/// nodes

//
// Display an individual node, as well as the global nodes in the left
// column.
//
App.NodesShowRoute = App.BaseRoute.extend({
  //
  // Set the model on the route. We look up the specific node
  // by it's identifier passed via the route
  //
  model: function(params) {
    return App.Node.create(window.fixtures.nodes_full[params.name]);
  },

  setupController: function(controller, model) {
      controller.set('content', model);
      //
      // Since we have 2 column layout, we need to also display the
      // list of nodes on the left. Hence setting the attribute
      // {{nodes}} on the controller.
      //
      controller.set('nodes', [App.Node.create(window.fixtures.nodes[0]), App.Node.create(window.fixtures.nodes[1])]);
  }
});

//
// Display all the nodes, allow to drill down into the specific nodes.
//
App.NodesRoute = App.BaseRoute.extend({
  //
  // Set the node as the routes default model to be called in
  // the template as {{model}}. This is the "expanded" view.
  //
  setupController: function(controller, model) {
      //
      // Since we have 2 column layout, we need to also display the
      // list of nodes on the left. Hence setting the attribute
      // {{nodes}} on the controller.
      //
      controller.set('nodes', [App.Node.create(window.fixtures.nodes[0]), App.Node.create(window.fixtures.nodes[1])]);
  }
});
