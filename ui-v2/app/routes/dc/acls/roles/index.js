import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithRoleActions from 'consul-ui/mixins/role/with-actions';

export default Route.extend(WithRoleActions, {
  repo: service('repository/role'),
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  model: function(params) {
    const repo = get(this, 'repo');
    return hash({
      ...repo.status({
        items: repo.findAllByDatacenter(this.modelFor('dc').dc.Name),
      }),
      isLoading: false,
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
