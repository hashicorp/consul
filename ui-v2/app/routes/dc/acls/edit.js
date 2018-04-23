import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';
import WithFeedback from 'consul-ui/mixins/with-feedback';
export default Route.extend(WithFeedback, {
  repo: service('acls'),
  model: function(params) {
    return hash({
      item: get(this, 'repo').findBySlug(params.id, this.modelFor('dc').dc.Name),
      types: ['management', 'client'],
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    update: function(item) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .persist(item)
            .then(function() {
              return this.transitionTo('dc.acls');
            });
        },
        `Your ACL token was saved.`,
        `There was an error saving your ACL token.`
      );
    },
    delete: function(item) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .remove(item)
            .then(() => {
              return this.transitionTo('dc.acls');
            });
        },
        `Your ACL token was deleted.`,
        `There was an error deleting your ACL token.`
      );
    },
    cancel: function(item) {
      this.transitionTo('dc.acls');
    },
    use: function(item) {
      get(this, 'feedback').execute(
        () => {
          // settings.set('settings.token', acl.ID);
          this.transitionTo('dc.services');
        },
        `Now using new ACL token`,
        `There was an error using that ACL token`
      );
    },
    clone: function(item) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .clone(item)
            .then(item => {
              this.transitionTo('dc.acls.show', get(item, 'ID'));
            });
        },
        `Your ACL token was cloned.`,
        `There was an error cloning your ACL token.`
      );
    },
  },
});
