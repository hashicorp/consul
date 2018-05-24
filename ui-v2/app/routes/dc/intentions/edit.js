import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithAclActions from 'consul-ui/mixins/intention/with-actions';

export default Route.extend(WithAclActions, {
  repo: service('intentions'),
  servicesRepo: service('services'),
  model: function(params) {
    return hash({
      isLoading: false,
      item: get(this, 'repo').findBySlug(params.id, this.modelFor('dc').dc.Name),
      items: get(this, 'servicesRepo').findAllByDatacenter(this.modelFor('dc').dc.Name),
      intents: ['allow', 'deny'],
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
