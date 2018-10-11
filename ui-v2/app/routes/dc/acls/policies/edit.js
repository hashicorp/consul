import SingleRoute from 'consul-ui/routing/single';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithPolicyActions from 'consul-ui/mixins/policy/with-actions';

export default SingleRoute.extend(WithPolicyActions, {
  repo: service('policies'),
  tokensRepo: service('tokens'),
  datacenterRepo: service('dc'),
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    const tokensRepo = get(this, 'tokensRepo');
    return this._super(...arguments).then(model => {
      return hash({
        ...model,
        ...{
          datacenters: get(this, 'datacenterRepo').findAll(),
        },
        ...tokensRepo.status({
          items: tokensRepo.findByPolicy(get(model.item, 'ID'), dc),
        }),
      });
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
