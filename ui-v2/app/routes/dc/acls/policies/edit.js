import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithPolicyActions from 'consul-ui/mixins/policy/with-actions';

export default Route.extend(WithPolicyActions, {
  repo: service('policies'),
  tokenRepo: service('tokens'),
  settings: service('settings'),
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    return hash({
      isLoading: false,
      item: get(this, 'repo').findBySlug(params.id, dc),
    }).then(model => {
      return hash({
        ...model,
        ...{
          items: get(this, 'tokenRepo').findByPolicy(get(model.item, 'ID'), dc),
        },
      });
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
