import SingleRoute from 'consul-ui/routing/single';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithPolicyActions from 'consul-ui/mixins/policy/with-actions';

export default SingleRoute.extend(WithPolicyActions, {
  repo: service('repository/policy'),
  tokenRepo: service('repository/token'),
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    const tokenRepo = get(this, 'tokenRepo');
    return this._super(...arguments).then(model => {
      return hash({
        ...model,
        ...{
          items: tokenRepo.findByPolicy(get(model.item, 'ID'), dc).catch(function(e) {
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
    controller.setProperties(model);
  },
});
