import Service from '@ember/service';
import { env } from 'consul-ui/env';

export default Service.extend({
  // deprecated
  // TODO: Remove this elsewhere in the app and use var instead
  env: function(key) {
    return env(key);
  },
  var: function(key) {
    return env(key);
  },
});
