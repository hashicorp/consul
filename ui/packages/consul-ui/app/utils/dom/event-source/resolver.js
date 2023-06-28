/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (P = Promise) {
  return function (source, listeners) {
    let current;
    if (typeof source.getCurrentEvent === 'function') {
      current = source.getCurrentEvent();
    }
    if (current != null) {
      // immediately resolve if we have previous cached data
      return P.resolve(current.data).then(function (cached) {
        source.open();
        return cached;
      });
    }
    // if we have no previously cached data, listen for the first response
    return new P(function (resolve, reject) {
      // close, cleanup and reject if we get an error
      listeners.add(source, 'error', function (e) {
        listeners.remove();
        e.target.close();
        reject(e.error);
      });
      // ...or cleanup and respond with the first lot of data
      listeners.add(source, 'message', function (e) {
        listeners.remove();
        resolve(e.data);
      });
    });
  };
}
