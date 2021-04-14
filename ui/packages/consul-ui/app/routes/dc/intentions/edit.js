import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';

export default class EditRoute extends Route {
  @service('repository/intention') repo;
  @service('env') env;

  async model(params, transition) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.optionalParams().nspace;

    let item;
    if (typeof params.intention_id !== 'undefined') {
      item = await this.repo.findBySlug({
        ns: nspace,
        dc: dc,
        id: params.intention_id,
      });
    } else {
      const defaultNspace = this.env.var('CONSUL_NSPACES_ENABLED') ? '*' : 'default';
      item = await this.repo.create({
        SourceNS: nspace || defaultNspace,
        DestinationNS: nspace || defaultNspace,
        Datacenter: dc,
      });
    }
    return {
      dc,
      nspace,
      item,
    };
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
