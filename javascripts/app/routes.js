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
  model: function(params) {
    return Ember.$.getJSON('/v1/catalog/datacenters').then(function(data) {
      return data
    });
  },

  setupController: function(controller, model) {
    controller.set('content', model);
    controller.set('dcs', model);
  },

  afterModel: function(dcs, transition) {
    if (dcs.get('length') === 1) {
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
    var object = Ember.Object.create();
    object.set('dc', params.dc)

    var nodesPromise =  Ember.$.getJSON('/v1/internal/ui/nodes').then(function(data) {
      objs = [];

      data.map(function(obj){
        objs.push(App.Node.create(obj));
      });

      object.set('nodes', objs);
      return object;
    });

    var datacentersPromise = Ember.$.getJSON('/v1/catalog/datacenters').then(function(data) {
      object.set('dcs', data);
      return object;
    });

    return nodesPromise.then(datacentersPromise);
  },

  setupController: function(controller, model) {
    controller.set('content', model.get('dc'));
    controller.set('nodes', model.get('nodes'));
    controller.set('dcs', model.get('dcs'));
  }
});


App.KvIndexRoute = App.BaseRoute.extend({
  beforeModel: function() {
    this.transitionTo('kv.show', '-')
  }
});

App.KvShowRoute = App.BaseRoute.extend({
  model: function(params) {
    var key = params.key.replace(/-/g, "/")

    return Ember.$.getJSON('/v1/kv/' + key + '?keys&seperator=' + '/').then(function(data) {

      objs = [];

      data.map(function(obj){
       objs.push(App.Key.create({Key: obj}));
      });

      return objs;
    });
  },

  setupController: function(controller, model) {
    controller.set('content', model);
    controller.set('topModel', model[0]);
    controller.set('newKey', App.Key.create());
  }
});

App.KvEditRoute = App.BaseRoute.extend({
  model: function(params) {
    var object = Ember.Object.create();
    var keyName = params.key.replace(/-/g, "/")
    var key = keyName;
    var parentKey;

    // Get the parent key
    if (key.slice(-1) == "/") {
      key = key.substring(0, key.length - 1);
    }
    parts = key.split('/');
    parts.pop();
    if (parts.length == 0) {
      parentKey = ""
    } else {
      parentKey = parts.join("/") + "/";
    }

    var keyPromise = Ember.$.getJSON('/v1/kv/' + keyName).then(function(data) {
      object.set('key', App.Key.create().setProperties(data[0]))
      return object;
    });

    var keysPromise = Ember.$.getJSON('/v1/kv/' + parentKey + '?keys&seperator=' + '/').then(function(data) {
      objs = [];
      data.map(function(obj){
       objs.push(App.Key.create({Key: obj}));
      });
      object.set('keys', objs);
      return object;
    });

    return keysPromise.then(keyPromise);
  },

  setupController: function(controller, model) {
    controller.set('content', model.get('key'));
    controller.set('siblings', model.get('keys'));

    if (this.modelFor('kv.show') == undefined ) {
    } else {
      controller.set('siblings', this.modelFor('kv.show'));
    }
  }
});

/// services

//
// Display all the services, allow to drill down into the specific services.
//
App.ServicesRoute = App.BaseRoute.extend({
  model: function(params) {
    return Ember.$.getJSON('/v1/internal/ui/services').then(function(data) {
      objs = [];
      data.map(function(obj){
       objs.push(App.Service.create(obj));
      });
      return objs
    });
  },
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
    controller.set('services', model);
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
    return Ember.$.getJSON('/v1/health/service/' + params.name).then(function(data) {
      objs = [];

      data.map(function(obj){
       objs.push(App.Node.create(obj));
      });

      console.log(objs)
      return objs;
    });
  },
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
    return Ember.RSVP.hash({
      node: Ember.$.getJSON('/v1/internal/ui/node/' + params.name).then(function(data) {
        return App.Node.create(data)
      }),
      nodes: Ember.$.getJSON('/v1/internal/ui/node/' + params.name).then(function(data) {
        return App.Node.create(data)
      })
    });

  },

  setupController: function(controller, models) {
      controller.set('content', models.node);
      //
      // Since we have 2 column layout, we need to also display the
      // list of nodes on the left. Hence setting the attribute
      // {{nodes}} on the controller.
      //
      controller.set('nodes', models.nodes);
  }
});

//
// Display all the nodes, allow to drill down into the specific nodes.
//
App.NodesRoute = App.BaseRoute.extend({

  model: function(params) {
    return Ember.$.getJSON('/v1/internal/ui/nodes').then(function(data) {
      objs = [];
      data.map(function(obj){
       objs.push(App.Node.create(obj));
      });
      return objs
    });
  },
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
      controller.set('nodes', model);
  }
});
