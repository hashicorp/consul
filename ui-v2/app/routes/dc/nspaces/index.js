import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

import WithNspaceActions from 'consul-ui/mixins/nspace/with-actions';
export default Route.extend(WithNspaceActions, {
  data: service('data-source/service'),
  repo: service('repository/nspace'),
  queryParams: {
    search: {
      as: 'filter',
      replace: true,
    },
  },
  model: function(params) {
    return hash({
      items: this.data.source(uri => uri`/*/*/namespaces`),
      isLoading: false,
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
