import Service, { inject as service } from '@ember/service';
import { setProperties } from '@ember/object';

export default Service.extend({
  settings: service('settings'),
  prepare: function(sink, data, instance = {}) {
    if (data === null || data === '') {
      return instance;
    }
    return setProperties(instance, data);
  },
  persist: function(sink, instance) {
    const slug = sink.split(':').pop();
    const repo = this.settings;
    return repo.persist({
      [slug]: instance,
    });
  },
  remove: function(sink, instance) {
    const slug = sink.split(':').pop();
    const repo = this.settings;
    return repo.delete(slug);
  },
});
