import { inject as service } from '@ember/service';
import SingleRoute from 'consul-ui/routing/single';
import { hash } from 'rsvp';

export default class ShowRoute extends SingleRoute {
  @service('repository/auth-method') repo;
  @service('repository/binding-rule') bindingRuleRepo;

  model(params) {
    const dc = this.modelFor('dc').dc;
    const nspace = this.optionalParams().nspace;

    return super.model(...arguments).then(model => {
      return hash({
        ...model,
        ...{
          item: this.repo.findBySlug({
            id: params.id,
            dc: dc.Name,
            ns: nspace,
          }),
          bindingRules: this.bindingRuleRepo.findAllByDatacenter({
            ns: nspace,
            dc: dc.Name,
            authmethod: params.id,
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
