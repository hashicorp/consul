import EmberRouter from '@ember/routing/router';
import config from './config/environment';

const Router = EmberRouter.extend({
  location: config.locationType,
  rootURL: config.rootURL,
});
Router.map(function() {
  // Our parent datacenter resource sets the namespace
  // for the entire application
  this.route('dc', { path: '/:dc' }, function() {
    // Services represent a consul service
    this.route('services', { path: '/services' }, function() {
      // Show an individual service
      this.route('show', { path: '/*name' });
    });
    // Nodes represent a consul node
    this.route('nodes', { path: '/nodes' }, function() {
      // Show an individual node
      this.route('show', { path: '/:name' });
    });
    // Intentions represent a consul intention
    this.route('intentions', { path: '/intentions' }, function() {
      this.route('edit', { path: '/:id' });
      this.route('create', { path: '/create' });
    });
    // Key/Value
    this.route('kv', { path: '/kv' }, function() {
      this.route('folder', { path: '/*key' });
      this.route('edit', { path: '/*key/edit' });
      this.route('create', { path: '/*key/create' });
      this.route('root-create', { path: '/create' });
    });
    // ACLs
    this.route('acls', { path: '/acls' }, function() {
      this.route('edit', { path: '/:id' });
      this.route('create', { path: '/create' });
    });
  });

  // Shows a datacenter picker. If you only have one
  // it just redirects you through.
  this.route('index', { path: '/' });

  // The settings page is global.
  this.route('settings', { path: '/settings' });
  this.route('notfound', { path: '/*path' });
});

export default Router;
