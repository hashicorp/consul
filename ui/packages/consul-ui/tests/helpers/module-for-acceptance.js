/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module } from 'qunit';
import { resolve } from 'rsvp';
import startApp from '../helpers/start-app';
import destroyApp from '../helpers/destroy-app';

export default function (name, options = {}) {
  let setTimeout = window.setTimeout;
  let setInterval = window.setInterval;
  module(name, {
    beforeEach() {
      const speedup = function (func) {
        return function (cb, interval = 0) {
          if (interval > 10) {
            interval = Math.max(Math.round(interval / 10), 10);
          }
          return func(cb, interval);
        };
      };
      window.setTimeout = speedup(window.setTimeout);
      window.setInterval = speedup(window.setInterval);
      this.application = startApp();

      if (options.beforeEach) {
        return options.beforeEach.apply(this, arguments);
      }
    },

    afterEach() {
      window.setTimeout = setTimeout;
      window.setInterval = setInterval;
      let afterEach = options.afterEach && options.afterEach.apply(this, arguments);
      return resolve(afterEach).then(() => destroyApp(this.application));
    },
  });
}
