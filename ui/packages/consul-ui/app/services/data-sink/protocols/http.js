import Service, { inject as service } from '@ember/service';
import { setProperties } from '@ember/object';

export default class HttpService extends Service {
  @service('settings') settings;
  @service('repository/intention') intention;
  @service('repository/kv') kv;
  @service('repository/session') session;

  prepare(sink, data, instance) {
    return setProperties(instance, data);
  }

  persist(sink, instance) {
    const [, , , , model] = sink.split('/');
    const repo = this[model];
    return repo.persist(instance);
  }

  remove(sink, instance) {
    const [, , , , model] = sink.split('/');
    const repo = this[model];
    return repo.remove(instance);
  }
}
