import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

import WithPolicyActions from 'consul-ui/mixins/policy/with-actions';

export default Route.extend(WithPolicyActions, {
  repo: service('repository/policy'),
  queryParams: {
    sortBy: 'sort',
    search: {
      as: 'filter',
      replace: true,
    },
  },
  model: function(params) {
    return hash({
      ...this.repo.status({
        items: this.repo.findAllByDatacenter(
          this.modelFor('dc').dc.Name,
          this.modelFor('nspace').nspace.substr(1)
        ),
      }),
      isLoading: false,
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
