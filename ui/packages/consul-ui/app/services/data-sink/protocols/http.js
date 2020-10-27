import Service, { inject as service } from '@ember/service';
import { setProperties } from '@ember/object';

export default Service.extend({
  settings: service('settings'),
  intention: service('repository/intention'),
  kv: service('repository/kv'),
  session: service('repository/session'),
  prepare: function(sink, data, instance) {
    return setProperties(instance, data);
  },
  persist: function(sink, instance) {
    const [, , , model] = sink.split('/');
    const repo = this[model];
    return repo.persist(instance);
  },
  remove: function(sink, instance) {
    const [, , , model] = sink.split('/');
    const repo = this[model];
    return repo.remove(instance);
  },
});
