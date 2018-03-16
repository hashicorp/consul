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
    update: function(item) {
      this.get('feedback').execute(
        () => {
          return this.get('repo').persist(item, this.modelFor('dc').dc);
        },
        `Updated ${item.get('ID')}`,
        `There was an error updating ${item.get('ID')}`
      );
    },
    delete: function(item) {
      this.get('feedback').execute(
        () => {
          return this.get('repo')
            .remove(item)
            .then(() => {
              this.transitionTo('dc.acls');
            });
        },
        `Deleted ${item.get('ID')}`,
        `There was an error deleting ${item.get('ID')}`
      );
    },
    use: function(item) {
      this.get('feedback').execute(
        () => {
          // settings.set('settings.token', acl.ID);
          this.transitionTo('dc.services');
        },
        `Now using ${item.get('ID')}`,
        `There was an error using ${item.get('ID')}`
      );
    },
    clone: function(item) {
      this.get('feedback').execute(
        () => {
          return this.get('repo')
            .clone(item, this.modelFor('dc').dc)
            .then(item => {
              this.transitionTo('dc.acls.show', item.get('ID'));
            });
        },
        `Cloned ${item.get('ID')}`,
        `There was an error cloning ${item.get('ID')}`
      );
    },
  },
});
