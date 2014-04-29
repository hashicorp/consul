window.App = Ember.Application.create({
  rootElement: "#app",
  LOG_TRANSITIONS: true
});

App.Router.map(function() {

  this.resource("dc", {path: "/:dc"}, function() {
    this.resource("services", { path: "/services" }, function(){
      this.route("show", { path: "/:name" });
    });
    this.resource("nodes", { path: "/nodes" }, function() {
      this.route("show", { path: "/:name" });
    });
    this.resource("kv", { path: "/kv" }, function(){
      this.route("index", { path: "/" });
      this.route("show", { path: "/:key" });
      this.route("edit", { path: "/:key/edit" });
    })
  });

  this.route("index", { path: "/" });
});

