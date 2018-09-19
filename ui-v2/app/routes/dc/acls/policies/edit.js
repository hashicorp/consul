import SingleRoute from 'consul-ui/routing/single';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithPolicyActions from 'consul-ui/mixins/policy/with-actions';

export default SingleRoute.extend(WithPolicyActions, {
  repo: service('policies'),
  tokenRepo: service('tokens'),
  datacenterRepo: service('dc'),
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    return this._super(...arguments).then(model => {
      return hash({
        ...model,
        ...{
          items: get(this, 'tokenRepo').findByPolicy(get(model.item, 'ID'), dc),
          datacenters: get(this, 'datacenterRepo').findAll(),
        },
      });
    });
  },
});
