import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get, set } from '@ember/object';

import WithTokenActions from 'consul-ui/mixins/token/with-actions';

export default Route.extend(WithTokenActions, {
  templateName: 'dc/acls/tokens/edit',
  repo: service('tokens'),
  beforeModel: function() {
    get(this, 'repo').invalidate();
  },
  model: function(params) {
    this.item = get(this, 'repo').create();
    set(this.item, 'Datacenter', this.modelFor('dc').dc.Name);
    return hash({
      create: true,
      isLoading: false,
      item: this.item,
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
  deactivate: function() {
    if (get(this.item, 'isNew')) {
      this.item.destroyRecord();
    }
  },
});
