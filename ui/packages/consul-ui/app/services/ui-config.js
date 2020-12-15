import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';

export default class UiConfigService extends Service {
  @service('env') env;

  async findByPath(path, configuration = {}) {
    return get(this.get(), path);
  }

  get() {
    return this.env.var('CONSUL_UI_CONFIG');
  }
}
