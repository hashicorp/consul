import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

import WithNspaceActions from 'consul-ui/mixins/nspace/with-actions';
export default Route.extend(WithNspaceActions, {
  data: service('data-source/service'),
  repo: service('repository/nspace'),
  queryParams: {
    sortBy: 'sort',
    search: {
      as: 'filter',
      replace: true,
    },
  },
  model: function(params) {
    return hash({
      items: this.data.source(uri => uri`/*/*/namespaces`),
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
