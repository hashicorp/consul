import SingleRoute from 'consul-ui/routing/single';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithTokenActions from 'consul-ui/mixins/token/with-actions';

export default SingleRoute.extend(WithTokenActions, {
  repo: service('tokens'),
  policiesRepo: service('policies'),
  model: function(params, transition) {
    const dc = this.modelFor('dc').dc.Name;
    return this._super(...arguments).then(model => {
      return hash({
        ...model,
        ...{
          items: get(this, 'policiesRepo').findAllByDatacenter(dc),
        },
      });
    });
  },
});
