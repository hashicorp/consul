import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import handle from 'consul-ui/utils/handle';
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
      handle.bind(this)(
        () => {
          // settings.set('settings.token', acl.ID);
          this.transitionTo('dc.services');
        },
        'using',
        'errored'
      );
    },
    clone: function(acl) {
      handle.bind(this)(
        () => {
          return this.get('repo')
            .clone(acl, this.modelFor('dc').dc)
            .then(acl => {
              this.transitionTo('dc.acls.show', acl.get('ID'));
            });
        },
        'cloned',
        'errored'
      );
    },
    delete: function(acl) {
      handle.bind(this)(
        () => {
          return this.get('repo')
            .remove(acl, this.modelFor('dc').dc)
            .then(() => {
              this.transitionTo('dc.acls');
            });
        },
        'deleted',
        'errored'
      );
    },
    update: function(acl) {
      handle.bind(this)(
        () => {
          return this.get('repo').persist(acl, this.modelFor('dc').dc);
        },
        'Updated',
        'errored'
      );
    },
  },
});
