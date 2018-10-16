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
          items: tokensRepo.findByPolicy(get(model.item, 'ID'), dc).catch(function(e) {
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
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
