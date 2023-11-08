/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';

const pause = 2000;
// native EventSource retry is ~3s wait
// any specified errors here will mean that the blocking query will attempt
// a reconnection every 3s until it reconnects to Consul
export const createErrorBackoff = function (ms = 3000, P = Promise, wait = setTimeout) {
  // This expects an ember-data like error
  return function (err) {
    // expect and ember-data error or a http-like error (e.statusCode)
    let status = get(err, 'errors.firstObject.status') || get(err, 'statusCode');
    if (typeof status !== 'undefined') {
      // ember-data errors are strings, http errors are numbers
      status = status.toString();
      switch (true) {
        // Any '5xx' (not 500) errors should back off and try again
        case status.indexOf('5') === 0 && status.length === 3 && status !== '500':
        // fallsthrough
        case status === '0':
          // TODO: Move this to the view layer so we can show a connection error
          // and reconnection success to the user
          // Any 0 aborted connections should back off and try again
          return new P(function (resolve) {
            wait(function () {
              resolve(err);
            }, ms);
          });
      }
    }
    // any other errors should throw to be picked up by an error listener/catch
    throw err;
  };
};
export const validateCursor = function (current, prev = null) {
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
const throttle = function (configuration, prev, current) {
  return function (obj) {
    return new Promise(function (resolve, reject) {
      setTimeout(function () {
        resolve(obj);
      }, configuration.interval || pause);
    });
  };
};
const defaultCreateEvent = function (result, configuration) {
  return {
    type: 'message',
    data: result,
  };
};
/**
 * Wraps an EventSource with functionality to add native EventSource-like functionality
 *
 * @param {Class} [CallableEventSource] - CallableEventSource Class
 * @param {Function} [backoff] - Default backoff function for all instances, defaults to createErrorBackoff
 */
export default function (EventSource, backoff = createErrorBackoff()) {
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
  const BlockingEventSource = function (source, configuration = {}) {
    const { currentEvent, ...config } = configuration;
    EventSource.apply(this, [
      (configuration) => {
        const { createEvent, ...superConfiguration } = configuration;
        return source
          .apply(this, [superConfiguration, this])
          .catch(backoff)
          .then((result) => {
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
              configuration.cacheControl = meta.cacheControl;
              configuration.interval = meta.interval;
            }
            if ((configuration.cacheControl || '').indexOf('no-store') === -1) {
              this.currentEvent = event;
            }
            this.dispatchEvent(event);
            const throttledResolve = throttle(configuration, event, this.previousEvent);
            this.previousEvent = this.currentEvent;
            return throttledResolve(result);
          });
      },
      config,
    ]);
    if (typeof currentEvent !== 'undefined') {
      this.currentEvent = currentEvent;
    }
    // only on initialization
    // if we already have an currentEvent set via configuration
    // dispatch the event so things are populated immediately
    this.addEventListener('open', (e) => {
      const currentEvent = e.target.getCurrentEvent();
      if (typeof currentEvent !== 'undefined') {
        this.dispatchEvent(currentEvent);
      }
    });
  };
  BlockingEventSource.prototype = Object.assign(
    Object.create(EventSource.prototype, {
      constructor: {
        value: EventSource,
        configurable: true,
        writable: true,
      },
    }),
    {
      // if we are having these props, at least make getters
      getCurrentEvent: function () {
        return this.currentEvent;
      },
      getPreviousEvent: function () {
        return this.previousEvent;
      },
    }
  );
  return BlockingEventSource;
}
