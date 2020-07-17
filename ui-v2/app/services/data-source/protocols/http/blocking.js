import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';

import { BlockingEventSource as EventSource } from 'consul-ui/utils/dom/event-source';
import { ifNotBlocking } from 'consul-ui/services/settings';
import { restartWhenAvailable } from 'consul-ui/services/client/http';
import maybeCall from 'consul-ui/utils/maybe-call';

export default Service.extend({
  client: service('client/http'),
  settings: service('settings'),
  source: function(find, configuration) {
    return new EventSource((configuration, source) => {
      const close = source.close.bind(source);
      const deleteCursor = () => (configuration.cursor = undefined);
      //
      return maybeCall(deleteCursor, ifNotBlocking(this.settings))().then(() => {
        return find(configuration)
          .then(maybeCall(close, ifNotBlocking(this.settings)))
          .then(function(res) {
            if (typeof get(res || {}, 'meta.cursor') === 'undefined') {
              close();
            }
            return res;
          })
          .catch(restartWhenAvailable(this.client));
      });
    }, configuration);
  },
});
