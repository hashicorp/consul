import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';
import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default class SessionsRoute extends Route.extend(WithBlockingActions) {
  @service('data-source/service')
  data;

  @service('repository/session')
  sessionRepo;

  @service('feedback')
  feedback;

  model(params) {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.optionalParams().nspace;
    const node = this.paramsFor(parent).name;
    return hash({
      dc: dc,
      nspace: nspace,
      node: node,
      sessions: this.data.source(uri => uri`/${nspace}/${dc}/sessions/for-node/${node}`),
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }

  @action
  invalidateSession(item) {
    const route = this;
    return this.feedback.execute(() => {
      return this.sessionRepo.remove(item).then(() => {
        route.refresh();
      });
    }, 'delete');
  }
}
