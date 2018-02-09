import EmberRouter from '@ember/routing/router';
import config from './config/environment';

const Router = EmberRouter.extend({
  location: config.locationType,
  rootURL: config.rootURL
});
Router.map(function() {
  // Our parent datacenter resource sets the namespace
  // for the entire application
  this.route("dc", {path: "/:dc"}, function() {
    // Services represent a consul service
    this.route("services", { path: "/services" }, function() {
      // Show an individual service
      this.route("show", { path: "/*name" });
    });
    // Nodes represent a consul node
    this.route("nodes", { path: "/nodes" }, function() {
      // Show an individual node
      this.route("show", { path: "/:name" });
    });
    // Key/Value
    this.route("kv", { path: "/kv" }, function(){
      this.route("index", { path: "/" });
      // List keys. This is more like an index
      this.route("show", { path: "/*key" });
      // Edit a specific key
      this.route("edit", { path: "/*key/edit" });
    });
    // ACLs
    this.route("acls", { path: "/acls" }, function(){
      this.route("show", { path: "/:id" });
    });

    // Shows a page explaining that ACLs haven't been set-up
    this.route("aclsdisabled", { path: "/aclsdisabled" });

    // Shows a page explaining that the ACL token being used isn't
    // authorized
    this.route("unauthorized", { path: "/unauthorized" });
  });

  // Shows a datacenter picker. If you only have one
  // it just redirects you through.
  this.route("index", { path: "/" });

  // The settings page is global.
  this.resource("settings", { path: "/settings" });

});

export default Router;
