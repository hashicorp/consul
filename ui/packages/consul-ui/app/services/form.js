import Service, { inject as service } from '@ember/service';
import builderFactory from 'consul-ui/utils/form/builder';
const builder = builderFactory();
export default class FormService extends Service {
  // a `get` method is added via the form initializer
  // see initializers/form.js

  // TODO: Temporarily add these here until something else needs
  // dynamic repos
  @service('repository/role')
  role;

  @service('repository/policy')
  policy;

  //
  init() {
    super.init(...arguments);
    this.forms = [];
  }

  build(obj, name) {
    return builder(...arguments);
  }

  form() {}
}
