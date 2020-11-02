import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';

export default class EditRoute extends Route {
  @service('repository/intention')
  repo;

  model({ intention_id }, transition) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = '*';
    return hash({
      dc: dc,
      nspace: nspace,
      item:
        typeof intention_id !== 'undefined'
          ? this.repo.findBySlug(intention_id, dc, nspace)
          : this.repo.create({
              Datacenter: dc,
            }),
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
