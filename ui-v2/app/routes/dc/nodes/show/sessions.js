import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default Route.extend(WithBlockingActions, {
  sessionRepo: service('repository/session'),
  feedback: service('feedback'),
  model: function() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    return this.modelFor(parent);
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    invalidateSession: function(item) {
      const dc = this.modelFor('dc').dc.Name;
      const nspace = this.modelFor('nspace').nspace.substr(1);
      const controller = this.controller;
      return this.feedback.execute(() => {
        return this.sessionRepo.remove(item).then(() => {
          return this.sessionRepo.findByNode(item.Node, dc, nspace).then(function(sessions) {
            controller.setProperties({
              sessions: sessions,
            });
          });
        });
      }, 'delete');
    },
  },
});
