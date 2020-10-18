import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default Route.extend(WithBlockingActions, {
  data: service('data-source/service'),
  sessionRepo: service('repository/session'),
  feedback: service('feedback'),
  model: function() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    const node = this.paramsFor(parent).name;
    return hash({
      dc: dc,
      nspace: nspace,
      node: node,
      sessions: this.data.source(uri => uri`/${nspace}/${dc}/sessions/for-node/${node}`),
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
  actions: {
    invalidateSession: function(item) {
      const route = this;
      return this.feedback.execute(() => {
        return this.sessionRepo.remove(item).then(() => {
          route.refresh();
        });
      }, 'delete');
    },
  },
});
