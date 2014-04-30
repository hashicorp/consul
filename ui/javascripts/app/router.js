window.App = Ember.Application.create({
  rootElement: "#app",
  LOG_TRANSITIONS: true,
  // The baseUrl for AJAX requests
  // TODO implement in AJAX logic
  baseUrl: 'http://localhost:8500'
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
      // This route just redirects to /-
      this.route("index", { path: "/" });
      // List keys. This is more like an index
      this.route("show", { path: "/:key" });
      // Edit a specific key
      this.route("edit", { path: "/:key/edit" });
    })
  });

  // Shows a datacenter picker. If you only have one
  // it just redirects you through.
  this.route("index", { path: "/" });
});

