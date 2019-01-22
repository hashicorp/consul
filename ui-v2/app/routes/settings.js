import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';
export default Route.extend(WithBlockingActions, {
  repo: service('settings'),
  dcRepo: service('repository/dc'),
  model: function(params) {
    return hash({
      item: get(this, 'repo').findAll(),
      dcs: get(this, 'dcRepo').findAll(),
    }).then(model => {
      return hash({
        ...model,
        ...{
          dc: get(this, 'dcRepo').getActive(null, model.dcs),
        },
      });
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
  // overwrite afterUpdate and afterDelete hooks
  // to avoid the default 'return to listing page'
  afterUpdate: function() {},
  afterDelete: function() {},
});
