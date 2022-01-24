import Service from '@ember/service';
import { env } from 'consul-ui/env';

export default class EnvService extends Service {
  // deprecated
  // TODO: Remove this elsewhere in the app and use var instead
  env(key) {
    return this.var(key);
  }

  var(key) {
    return env(key);
  }
}
