/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (source, DefaultEventSource, P = Promise) {
  return function (sources) {
    return function (cb, configuration) {
      const key = configuration.key;
      if (typeof sources[key] !== 'undefined' && configuration.settings.enabled) {
        if (typeof sources[key].configuration === 'undefined') {
          sources[key].configuration = {};
        }
        sources[key].configuration.settings = configuration.settings;
        return source(sources[key]);
      } else {
        const EventSource = configuration.type || DefaultEventSource;
        const eventSource = (sources[key] = new EventSource(cb, configuration));
        return source(eventSource)
          .catch(function (e) {
            // any errors, delete from the cache for next time
            delete sources[key];
            return P.reject(e);
          })
          .then(function (eventSource) {
            // make sure we cancel everything out if there is no cursor
            if (typeof eventSource.configuration.cursor === 'undefined') {
              eventSource.close();
              delete sources[key];
            }
            return eventSource;
          });
      }
    };
  };
}
