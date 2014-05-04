//
// Superclass to be used by all of the main routes below.
//
App.BaseRoute = Ember.Route.extend({
  rootRoute: '',

  getParentAndGrandparent: function(key) {
    var parentKey = this.rootRoute,
        grandParentKey = this.rootRoute,
        parts = key.split('/');

    if (parts.length > 0) {
      parts.pop();
      parentKey = parts.join("/") + "/";
    }

    if (parts.length > 1) {
      parts.pop();
      grandParentKey = parts.join("/") + "/";
    }

    return {grandParent: grandParentKey, parent: parentKey}
  },

  removeDuplicateKeys: function(keys, matcher) {
    // Loop over the keys
    keys.forEach(function(item, index) {
      if (item.get('Key') == matcher) {
      // If we are in a nested folder and the folder
      // name matches our position, remove it
        keys.splice(index, 1);
      }
    });
    return keys;
  },

  actions: {
    // Used to link to keys that are not objects,
    // like parents and grandParents
    linkToKey: function(key) {
      if (key.slice(-1) === '/' || key === this.rootRoute) {
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
App.IndexRoute = App.BaseRoute.extend({
  // Retrieve the list of datacenters
  model: function(params) {
    return Ember.$.getJSON('/v1/catalog/datacenters').then(function(data) {
      return data
    })
  },

  afterModel: function(model, transition) {
    // If we only have one datacenter, jump
    // straight to it and bypass the global
    // view
    if (model.get('length') === 1) {
      this.transitionTo('services', model[0]);
    }
  }
});

// The parent route for all resources. This keeps the top bar
// functioning, as well as the per-dc requests.
App.DcRoute = App.BaseRoute.extend({
  model: function(params) {
    // Return a promise hash to retreieve the
    // dcs and nodes used in the header
    return Ember.RSVP.hash({
      dc: params.dc,
      dcs: Ember.$.getJSON('/v1/catalog/datacenters'),
      nodes: Ember.$.getJSON('/v1/internal/ui/nodes?dc=' + params.dc).then(function(data) {
        objs = [];

        // Merge the nodes into a list and create objects out of them
        data.map(function(obj){
          objs.push(App.Node.create(obj));
        });

        return objs;
      })
    });
  },

  setupController: function(controller, models) {
    controller.set('content', models.dc);
    controller.set('nodes', models.nodes);
    controller.set('dcs', models.dcs);
    controller.set('isDropdownVisible', false);
  },
});

App.KvIndexRoute = App.BaseRoute.extend({
  beforeModel: function() {
    this.transitionTo('kv.show', this.rootRoute)
  }
});

App.KvShowRoute = App.BaseRoute.extend({
  model: function(params) {
    var key = params.key;
    var dc = this.modelFor('dc').dc;

    // Return a promise has with the ?keys for that namespace
    // and the original key requested in params
    return Ember.RSVP.hash({
      key: key,
      keys: Ember.$.getJSON('/v1/kv/' + key + '?keys&seperator=' + '/&dc=' + dc).then(function(data) {
        objs = [];
        data.map(function(obj){
          objs.push(App.Key.create({Key: obj}));
        });
        return objs;
      })
    });
  },

  setupController: function(controller, models) {
    var key = models.key;
    var parentKeys = this.getParentAndGrandparent(key);
    models.keys = this.removeDuplicateKeys(models.keys, models.key);

    controller.set('content', models.keys);
    controller.set('parentKey', parentKeys.parent);
    controller.set('grandParentKey', parentKeys.grandParent);
    controller.set('newKey', App.Key.create());
  }
});

App.KvEditRoute = App.BaseRoute.extend({
  model: function(params) {
    var key = params.key;
    var dc = this.modelFor('dc').dc;
    var parentKeys = this.getParentAndGrandparent(key)

    // Return a promise hash to get the data for both columns
    return Ember.RSVP.hash({
      key: Ember.$.getJSON('/v1/kv/' + key + '?dc=' + dc).then(function(data) {
        // Convert the returned data to a Key
        return App.Key.create().setProperties(data[0]);
      }),
      keys: keysPromise = Ember.$.getJSON('/v1/kv/' + parentKeys.parent + '?keys&seperator=' + '/' + '&dc=' + dc).then(function(data) {
        objs = [];
        data.map(function(obj){
         objs.push(App.Key.create({Key: obj}));
        });
        return objs;
      }),
    });
  },

  setupController: function(controller, models) {
    var key = models.key;
    var parentKeys = this.getParentAndGrandparent(key.get('Key'));
    models.keys = this.removeDuplicateKeys(models.keys, parentKeys.parent);

    controller.set('content', models.key);
    controller.set('parentKey', parentKeys.parent);
    controller.set('grandParentKey', parentKeys.grandParent);
    controller.set('siblings', models.keys);
  }
});

App.ServicesRoute = App.BaseRoute.extend({
  model: function(params) {
    var dc = this.modelFor('dc').dc
    // Return a promise to retrieve all of the services
    return Ember.$.getJSON('/v1/internal/ui/services?dc=' + dc).then(function(data) {
      objs = [];
      data.map(function(obj){
       objs.push(App.Service.create(obj));
      });
      return objs
    });
  },
  setupController: function(controller, model) {
    controller.set('services', model);
  }
});


App.ServicesShowRoute = App.BaseRoute.extend({
  model: function(params) {
    var dc = this.modelFor('dc').dc
    // Here we just use the built-in health endpoint, as it gives us everything
    // we need.
    return Ember.$.getJSON('/v1/health/service/' + params.name + '?dc=' + dc).then(function(data) {
      objs = [];
      data.map(function(obj){
       objs.push(App.Node.create(obj));
      });
      return objs;
    });
  }
});

App.NodesShowRoute = App.BaseRoute.extend({
  model: function(params) {
    var dc = this.modelFor('dc').dc
    // Return a promise hash of the node and nodes
    return Ember.RSVP.hash({
      node: Ember.$.getJSON('/v1/internal/ui/node/' + params.name + '?dc=' + dc).then(function(data) {
        return App.Node.create(data)
      }),
      nodes: Ember.$.getJSON('/v1/internal/ui/node/' + params.name + '?dc=' + dc).then(function(data) {
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

App.NodesRoute = App.BaseRoute.extend({
  model: function(params) {
    var dc = this.modelFor('dc').dc
    // Return a promise containing the nodes
    return Ember.$.getJSON('/v1/internal/ui/nodes?dc=' + dc).then(function(data) {
      objs = [];
      data.map(function(obj){
       objs.push(App.Node.create(obj));
      });
      return objs
    });
  },
  setupController: function(controller, model) {
      controller.set('nodes', model);
  }
});
