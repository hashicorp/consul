/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

/* eslint no-console: "off" */
export default function (scenario, assert, currentURL) {
  scenario
    .then('print the current url', function (url) {
      console.log(currentURL());
      return Promise.resolve();
    })
    .then('log the "$text"', function (text) {
      console.log(text);
      return Promise.resolve();
    })
    .then('pause for $milliseconds', function (milliseconds) {
      return new Promise(function (resolve) {
        setTimeout(resolve, milliseconds);
      });
    })
    .then('ok', function () {
      assert.ok(true);
    });
}
