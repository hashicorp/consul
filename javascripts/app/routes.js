//
// Superclass to be used by all of the main routes below. All routes
// but the IndexRoute share the need to have a datacenter set.
//
//
App.BaseRoute = Ember.Route.extend({
  actions: {
    linkToKey: function(key) {
      key = key.replace(/\//g, "-")

      if (key.slice(-1) === "-") {
        this.transitionTo('kv.show', key)
      } else {
        this.transitionTo('kv.edit', key)
      }
    }
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

  setupController: function(controller, model) {
    controller.set('content', model);
    controller.set('dcs', window.fixtures.dcs);
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
  },

  setupController: function(controller, model) {
    controller.set('content', model);

    controller.set('services', [App.Service.create(window.fixtures.services[0]), App.Service.create(window.fixtures.services[1])]);

    controller.set('dcs', window.fixtures.dcs);
  }
});


App.KvRoute = App.BaseRoute.extend({
  beforeModel: function() {
    this.transitionTo('kv.show', '-')
  }
});

App.KvShowRoute = App.BaseRoute.extend({
  model: function(params) {
    var key = params.key.replace(/-/g, "/")
    objs = [];

    window.fixtures.keys_full[key].map(function(obj){
     objs.push(App.Key.create({key: obj}));
    });

    return objs
  },

  setupController: function(controller, model) {
    controller.set('content', model);
    controller.set('parent', model[0].get('parentKeys'));
  }
});

App.KvEditRoute = App.BaseRoute.extend({
  model: function(params) {
    var key = params.key.replace(/-/g, "/")
    return App.Key.create().setProperties(window.fixtures.keys_full[key]);
  },

  setupController: function(controller, model) {
    controller.set('content', model);
    controller.set('siblings', this.modelFor('kv.show'));
    console.log(this.modelFor('kv.show'))
    controller.set('parent', model.get('parentKeys'));
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
