import { inject as service } from '@ember/service';
import SingleRoute from 'consul-ui/routing/single';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithPolicyActions from 'consul-ui/mixins/policy/with-actions';

export default class EditRoute extends SingleRoute.extend(WithPolicyActions) {
  @service('repository/policy')
  repo;

  @service('repository/token')
  tokenRepo;

  model(params) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.optionalParams().nspace;
    const tokenRepo = this.tokenRepo;
    return super.model(...arguments).then(model => {
      return hash({
        ...model,
        ...{
          routeName: this.routeName,
          items: tokenRepo
            .findByPolicy({
              ns: nspace,
              dc: dc,
              id: get(model.item, 'ID'),
            })
            .catch(function(e) {
              switch (get(e, 'errors.firstObject.status')) {
                case '403':
                case '401':
                  // do nothing the SingleRoute will have caught it already
                  return;
              }
              throw e;
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
