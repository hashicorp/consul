import Service, { inject as service } from '@ember/service';
import builderFactory from 'consul-ui/utils/form/builder';
const builder = builderFactory();
export default Service.extend({
  // a `get` method is added via the form initializer
  // see initializers/form.js

  // TODO: Temporarily add these here until something else needs
  // dynamic repos
  role: service('repository/role'),
  policy: service('repository/policy'),
  //
  init: function() {
    this._super(...arguments);
    this.forms = [];
  },
  build: function(obj, name) {
    return builder(...arguments);
  },
  form: function() {},
});
