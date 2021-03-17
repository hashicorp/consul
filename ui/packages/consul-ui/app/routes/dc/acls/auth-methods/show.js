import { inject as service } from '@ember/service';
import SingleRoute from 'consul-ui/routing/single';
import { hash } from 'rsvp';

export default class ShowRoute extends SingleRoute {
  @service('repository/auth-method') repo;

  model(params) {
    return super.model(...arguments).then(model => {
      return hash({
        ...model,
        ...{
          item: this.repo.findBySlug({
            id: params.id,
            dc: this.modelFor('dc').dc.Name,
            ns: this.modelFor('nspace').nspace.substr(1),
          }),
        },
      });
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
