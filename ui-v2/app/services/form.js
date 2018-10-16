import Service from '@ember/service';
import builderFactory from 'consul-ui/utils/form/builder';
const builder = builderFactory();
export default Service.extend({
  // a `get` method is added via the form initializer
  // see initializers/form.js
  build: function(obj, name) {
    return builder(...arguments);
  },
});
