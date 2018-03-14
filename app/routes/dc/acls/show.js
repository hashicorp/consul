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
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    use: function(acl) {
      this.get('feedback').execute(
        () => {
          // settings.set('settings.token', acl.ID);
          this.transitionTo('dc.services');
        },
        `Now using ${acl.get('ID')}`,
        `There was an error using ${acl.get('ID')}`
      );
    },
    clone: function(acl) {
      this.get('feedback').execute(
        () => {
          return this.get('repo')
            .clone(acl, this.modelFor('dc').dc)
            .then(acl => {
              this.transitionTo('dc.acls.show', acl.get('ID'));
            });
        },
        `Cloned ${acl.get('ID')}`,
        `There was an error cloning ${acl.get('ID')}`
      );
    },
    delete: function(acl) {
      this.get('feedback').execute(
        () => {
          return this.get('repo')
            .remove(acl, this.modelFor('dc').dc)
            .then(() => {
              this.transitionTo('dc.acls');
            });
        },
        `Deleted ${acl.get('ID')}`,
        `There was an error deleting ${acl.get('ID')}`
      );
    },
    update: function(acl) {
      this.get('feedback').execute(
        () => {
          return this.get('repo').persist(acl, this.modelFor('dc').dc);
        },
        `Updated ${acl.get('ID')}`,
        `There was an error updating ${acl.get('ID')}`
      );
    },
  },
});
