import Service, { inject as service } from '@ember/service';

import MultiMap from 'mnemonist/multi-map';

import { BlockingEventSource as EventSource } from 'consul-ui/utils/dom/event-source';
// utility functions to make the below code easier to read:
import { ifNotBlocking } from 'consul-ui/services/settings';
import { restartWhenAvailable } from 'consul-ui/services/client/http';
import maybeCall from 'consul-ui/utils/maybe-call';

// TODO: Expose sizes of things via env vars

// caches cursors and previous events when the EventSources are destroyed
const cache = new Map();
// keeps a record of currently in use EventSources
const sources = new Map();
// keeps a count of currently in use EventSources
const usage = new MultiMap(Set);

export default Service.extend({
  client: service('client/http'),
  settings: service('settings'),

  finder: service('repository/manager'),

  open: function(uri, ref) {
    let source;
    // Check the cache for an EventSource that is already being used
    // for this uri. If we don't have one, set one up.
    if (!sources.has(uri)) {
      // Setting up involves finding the correct repository method to call
      // based on the uri, we use the eventually injectable finder for this.
      const repo = this.finder.fromURI(...uri.split('?filter='));
      // We then check the to see if this we have previously cached information
      // for the URI. This data comes from caching this data when the EventSource
      // is closed and destroyed. We recreate the EventSource from the data from the cache
      // if so. The data is currently the cursor position and the last received data.
      let configuration = {};
      if (cache.has(uri)) {
        configuration = cache.get(uri);
      }
      // tag on the custom createEvent for ember-data
      configuration.createEvent = repo.reconcile;

      // We create the EventSource, checking to make sure whether we should close the
      // EventSource on first response (depending on the user setting) and reopening
      // the EventSource if it has been paused by the user navigating to another tab
      source = new EventSource((configuration, source) => {
        const close = source.close.bind(source);
        const deleteCursor = () => (configuration.cursor = undefined);
        //
        return maybeCall(deleteCursor, ifNotBlocking(this.settings))().then(() => {
          return repo
            .find(configuration)
            .then(maybeCall(close, ifNotBlocking(this.settings)))
            .catch(restartWhenAvailable(this.client));
        });
      }, configuration);
      // Lastly, when the EventSource is no longer needed, cache its information
      // for when we next need it again (see above re: data cache)
      source.addEventListener('close', function close(e) {
        const source = e.target;
        source.removeEventListener('close', close);
        cache.set(uri, {
          currentEvent: e.target.getCurrentEvent(),
          cursor: e.target.configuration.cursor,
        });
        // the data is cached delete the EventSource
        sources.delete(uri);
      });
      sources.set(uri, source);
    } else {
      source = sources.get(uri);
    }
    // set/increase the usage counter
    usage.set(source, ref);
    source.open();
    return source;
  },
  close: function(source, ref) {
    if (source) {
      // decrease the usage counter
      usage.remove(source, ref);
      // if the EventSource is no longer being used
      // close it (data caching is dealt with by the above 'close' event listener)
      if (!usage.has(source)) {
        source.close();
      }
    }
  },
});
