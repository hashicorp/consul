import Service, { inject as service } from '@ember/service';

import { CallableEventSource as EventSource } from 'consul-ui/utils/dom/event-source';

export default Service.extend({
  finder: service('repository/manager'),

  open: function(uri, ref) {
    const repo = this.finder.fromURI(...uri.split('?filter='));
    const source = new EventSource(function(configuration, source) {
      return repo.find(configuration).then(function(res) {
        source.dispatchEvent({ type: 'message', data: res });
        source.close();
      });
    }, {});
    return source;
  },
  close: function(source, ref) {
    if (source) {
      source.close();
    }
  },
});
