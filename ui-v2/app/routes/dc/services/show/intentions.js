import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import WithIntentionActions from 'consul-ui/mixins/intention/with-actions';

export default Route.extend(WithIntentionActions, {
  repo: service('repository/intention'),
  model: function() {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    return this.modelFor(parent);
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  // Overwrite default afterDelete action to just refresh
  afterDelete: function() {
    return this.refresh();
  },
});
