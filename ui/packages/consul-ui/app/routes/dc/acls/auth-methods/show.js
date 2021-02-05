import { inject as service } from '@ember/service';
import SingleRoute from 'consul-ui/routing/single';
import { hash } from 'rsvp';

export default class ShowRoute extends SingleRoute {
  @service('repository/auth-method') repo;

  model(params) {
    const dc = this.modelFor('dc').dc.Name;
    return super.model(...arguments).then(model => {
      return hash({
        ...model,
        ...{
          item: this.repo.findBySlug(params.id, dc),
        },
      });
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
