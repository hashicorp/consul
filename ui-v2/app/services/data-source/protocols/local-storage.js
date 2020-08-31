import Service, { inject as service } from '@ember/service';
import { StorageEventSource } from 'consul-ui/utils/dom/event-source';

export default Service.extend({
  repo: service('settings'),
  source: function(src, configuration) {
    const slug = src.split(':').pop();
    return new StorageEventSource(
      configuration => {
        return this.repo.findBySlug(slug);
      },
      {
        key: src,
        uri: configuration.uri,
      }
    );
  },
});
