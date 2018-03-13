import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('acls'),
  model: function(params) {
    const dc = this.modelFor('dc').dc;
    return hash({
      dc: dc,
      model: this.get('repo').findBySlug(params.id, dc),
      types: ['client', 'management'],
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    use: function(acl) {
      const controller = this.controller;
      controller.set('isLoading', true);
      const dc = this.modelFor('dc').dc;
      // settings.set('settings.token', acl.ID);
      controller.set('isLoading', false);
      // notify
      controller.transitionToRoute('dc.services');
    },
    clone: function(acl) {
      const controller = this.controller;
      controller.set('isLoading', true);
      const dc = this.modelFor('dc').dc;
      // Set
      // TODO: temporarily use clone for now
      this.get('repo')
        .clone(acl, dc)
        .then(function(acl) {
          controller.set('isLoading', false);
          // notify
          controller.transitionToRoute('acls.show', acl.get('ID'));
        })
        .catch(function(response) {
          // TODO: check e.errors
          // Render the error message on the form if the request failed
          controller.set('errorMessage', 'Received error while processing: ' + response.statusText);
        })
        .finally(function() {
          controller.set('isLoading', false);
        });
    },
    delete: function(acl) {
      const controller = this.controller;
      controller.set('isLoading', true);
      const dc = this.modelFor('dc').dc;
      this.get('repo')
        .remove(acl, dc)
        .then(function(response) {
          //transition, refresh?
        })
        .catch(function(response) {
          // Render the error message on the form if the request failed
          controller.set('errorMessage', 'Received error while processing: ' + response.statusText);
        })
        .finally(function() {
          controller.set('isLoading', false);
        });
    },
    update: function(acl) {
      var controller = this.controller;
      controller.set('isLoading', true);
      const dc = this.modelFor('dc').dc;

      // Update the ACL
      this.get('repo')
        .persist(acl, dc)
        .then(function() {
          // notify
        })
        .catch(function() {
          // notify
        })
        .finally(function() {
          controller.set('isLoading', false);
        });
    },
  },
});
