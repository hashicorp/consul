/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (P = Promise, timeout = setTimeout) {
  // var interval;
  return function (milliseconds, cb = function () {}) {
    // clearInterval(interval);
    // const cb = typeof _cb !== 'function' ? (i) => { clearInterval(interval);interval = i; } : _cb;
    return new P((resolve, reject) => {
      cb(
        timeout(function () {
          resolve(milliseconds);
        }, milliseconds)
      );
    });
  };
}
