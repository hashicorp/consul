window.App = Ember.Application.create({
  rootElement: "#app",
  currentPath: ''
});

Ember.Application.initializer({
  name: 'settings',

  initialize: function(container, application) {
    application.set('settings', App.Settings.create());
  }
});

// Wrap localstorage with an ember object
App.Settings = Ember.Object.extend({
  unknownProperty: function(key) {
    return localStorage[key];
  },

  setUnknownProperty: function(key, value) {
    if(Ember.isNone(value)) {
      delete localStorage[key];
    } else {
      localStorage[key] = value;
    }
    this.notifyPropertyChange(key);
    return value;
  },

  clear: function() {
    this.beginPropertyChanges();
    for (var i=0, l=localStorage.length; i<l; i++){
      this.set(localStorage.key(i));
    }
    localStorage.clear();
    this.endPropertyChanges();
  }
});

App.Router.map(function() {
  // Our parent datacenter resource sets the namespace
  // for the entire application
  this.resource("dc", {path: "/:dc"}, function() {
    // Services represent a consul service
    this.resource("services", { path: "/services" }, function(){
      // Show an individual service
      this.route("show", { path: "/:name" });
    });
    // Nodes represent a consul node
    this.resource("nodes", { path: "/nodes" }, function() {
      // Show an individual node
      this.route("show", { path: "/:name" });
    });
    // Key/Value
    this.resource("kv", { path: "/kv" }, function(){
      this.route("index", { path: "/" });
      // List keys. This is more like an index
      this.route("show", { path: "/*key" });
      // Edit a specific key
      this.route("edit", { path: "/*key/edit" });
    });
    // ACLs
    this.resource("acls", { path: "/acls" }, function(){
      this.route("show", { path: "/:name" });
    });

    // Shows a page explaining that ACLs haven't been set-up
    this.route("aclsdisabled", { path: "/aclsdisabled" });
    // Shows a page explaining that the ACL key being used isn't
    // authorized
    this.route("unauthorized", { path: "/unauthorized" });
  });

  // Shows a datacenter picker. If you only have one
  // it just redirects you through.
  this.route("index", { path: "/" });
});

