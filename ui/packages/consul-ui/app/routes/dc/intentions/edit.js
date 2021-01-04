import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';

export default class EditRoute extends Route {
  @service('repository/intention') repo;
  @service('env') env;

  async model({ intention_id }, transition) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);

    let item;
    if (typeof intention_id !== 'undefined') {
      item = await this.repo.findBySlug(intention_id, dc, nspace);
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
