import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default class EnvHelper extends Helper {
  @service('env') env;

  compute([name, def = ''], hash) {
    const val = this.env.var(name);
    return val != null ? val : def;
  }
}
