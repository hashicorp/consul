import { get } from '@ember/object';
import { Promise } from 'rsvp';

const pause = 2000;
// native EventSource retry is ~3s wait
export const create5xxBackoff = function(ms = 3000, P = Promise, wait = setTimeout) {
  // This expects an ember-data like error
  return function(err) {
    const status = get(err, 'errors.firstObject.status');
    if (typeof status !== 'undefined') {
      switch (true) {
        // Any '5xx' (not 500) errors should back off and try again
        case status.indexOf('5') === 0 && status.length === 3 && status !== '500':
          return new P(function(resolve) {
            wait(function() {
              resolve(err);
            }, ms);
          });
      }
    }
    // any other errors should throw to be picked up by an error listener/catch
    throw err;
  };
};
export const validateCursor = function(current, prev = null) {
  let cursor = parseInt(current);
  if (!isNaN(cursor)) {
    // if cursor is less than the current cursor, reset to zero
    if (prev !== null && cursor < prev) {
      cursor = 0;
    }
    // if cursor is less than 0, its always safe to use 1
    return Math.max(cursor, 1);
  }
};
const throttle = function(configuration, prev, current) {
  return function(obj) {
    return new Promise(function(resolve, reject) {
      setTimeout(function() {
        resolve(obj);
      }, pause);
    });
  };
};
const defaultCreateEvent = function(result, configuration) {
  return {
    type: 'message',
    data: result,
  };
};
/**
 * Wraps an EventSource with functionality to add native EventSource-like functionality
 *
 * @param {Class} [CallableEventSource] - CallableEventSource Class
 * @param {Function} [backoff] - Default backoff function for all instances, defaults to create5xxBackoff
 */
export default function(EventSource, backoff = create5xxBackoff()) {
  /**
   * An EventSource implementation to add native EventSource-like functionality with just callbacks (`cursor` and 5xx backoff)
   *
   * This includes:
   * 1. 5xx backoff support (uses a 3 second reconnect like native implementations). You can add to this via `Promise.catch`
   * 2. A `cursor` configuration value. Current `cursor` is taken from the `meta` property of the event (i.e. `event.data.meta.cursor`)
   * 3. Event data can be customized by adding a `configuration.createEvent`
   *
   * @param {Function} [source] - Promise returning function that resolves your data
   * @param {Object} [configuration] - Plain configuration object:
   *   `cursor` - Cursor position of the EventSource
   *   `createEvent` - A data filter, giving you the opportunity to filter or replace the event data, such as removing/replacing records
   */
  return class extends EventSource {
    constructor(source, configuration = {}) {
      const { currentEvent, ...config } = configuration;
      super(configuration => {
        const { createEvent, ...superConfiguration } = configuration;
        return source
          .apply(this, [superConfiguration, this])
          .catch(backoff)
          .then(result => {
            if (result instanceof Error) {
              return result;
            }
            const _createEvent =
              typeof createEvent === 'function' ? createEvent : defaultCreateEvent;
            let event = _createEvent(result, configuration);
            // allow custom types, but make a default of `message`, ideally this would check for CustomEvent
            // but keep this flexible for the moment
            if (!event.type) {
              event = {
                type: 'message',
                data: event,
              };
            }
            // meta is also configurable by using createEvent
            const meta = get(event.data || {}, 'meta');
            if (meta) {
              // pick off the `cursor` from the meta and add it to configuration
              // along with cursor validation
              configuration.cursor = validateCursor(meta.cursor, configuration.cursor);
            }
            this.currentEvent = event;
            this.dispatchEvent(this.currentEvent);
            const throttledResolve = throttle(configuration, this.currentEvent, this.previousEvent);
            this.previousEvent = this.currentEvent;
            return throttledResolve(result);
          });
      }, config);
      if (typeof currentEvent !== 'undefined') {
        this.currentEvent = currentEvent;
      }
      // only on initialization
      // if we already have an currentEvent set via configuration
      // dispatch the event so things are populated immediately
      this.addEventListener('open', e => {
        const currentEvent = e.target.getCurrentEvent();
        if (typeof currentEvent !== 'undefined') {
          this.dispatchEvent(currentEvent);
        }
      });
    }
    // if we are having these props, at least make getters
    getCurrentEvent() {
      return this.currentEvent;
    }
    getPreviousEvent() {
      return this.previousEvent;
    }
  };
}
