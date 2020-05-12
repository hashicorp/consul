import Service, { inject as service } from '@ember/service';
import { setProperties } from '@ember/object';

export default Service.extend({
  settings: service('settings'),
  intention: service('repository/intention'),
  prepare: function(sink, data, instance) {
    const [, dc, nspace, model, slug] = sink.split('/');
    const repo = this[model];
    if (slug === '') {
      instance = repo.create({
        Datacenter: dc,
        Namespace: nspace,
      });
    } else {
      if (typeof instance === 'undefined') {
        instance = repo.peek(slug);
      }
    }
    return setProperties(instance, data);
  },
  persist: function(sink, instance) {
    const [, , , /*dc*/ /*nspace*/ model] = sink.split('/');
    const repo = this[model];
    return repo.persist(instance);
  },
  remove: function(sink, instance) {
    const [, , , /*dc*/ /*nspace*/ model] = sink.split('/');
    const repo = this[model];
    return repo.remove(instance);
  },
});
