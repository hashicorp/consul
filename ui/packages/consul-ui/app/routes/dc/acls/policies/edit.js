import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default class EditRoute extends Route.extend(WithBlockingActions) {
  @service('repository/policy') repo;
}
