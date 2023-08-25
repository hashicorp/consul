/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

const dont = `( don't| shouldn't| can't)?`;
export default function (scenario, assert, pauseUntil, find, currentURL, clipboard) {
  scenario
    .then('pause until I see the text "$text" in "$selector"', function (text, selector) {
      return pauseUntil(function (resolve, reject, retry) {
        const $el = find(selector);
        if ($el) {
          const hasText = $el.textContent.indexOf(text) !== -1;
          if (hasText) {
            return resolve();
          }
          return reject();
        }
        return retry();
      }, `Expected to see "${text}" in "${selector}"`);
    })
    .then([`I${dont} see the text "$text" in "$selector"`], function (negative, text, selector) {
      const textContent = (find(selector) || { textContent: '' }).textContent;
      assert[negative ? 'notOk' : 'ok'](
        textContent.indexOf(text) !== -1,
        `Expected${negative ? ' not' : ''} to see "${text}" in "${selector}", was "${textContent}"`
      );
    })
    .then(['I copied "$text"'], function (text) {
      const copied = clipboard();
      assert.ok(
        copied.indexOf(text) !== -1,
        `Expected to see "${text}" in the clipboard, was "${copied}"`
      );
    })
    .then(['I see the exact text "$text" in "$selector"'], function (text, selector) {
      assert.ok(
        find(selector).textContent.trim() === text,
        `Expected to see the exact "${text}" in "${selector}"`
      );
    })
    // TODO: Think of better language
    // TODO: These should be mergeable
    .then(['"$selector" has the "$class" class'], function (selector, cls) {
      // because `find` doesn't work, guessing its sandboxed to ember's container
      assert
        .dom(document.querySelector(selector))
        .hasClass(cls, `Expected [class] to contain ${cls} on ${selector}`);
    })
    .then(['"$selector" doesn\'t have the "$class" class'], function (selector, cls) {
      assert.ok(
        !document.querySelector(selector).classList.contains(cls),
        `Expected [class] not to contain ${cls} on ${selector}`
      );
    })
    .then([`I${dont} see the "$selector" element`], function (negative, selector) {
      assert[negative ? 'equal' : 'notEqual'](
        document.querySelector(selector),
        null,
        `Expected${negative ? ' not' : ''} to see ${selector}`
      );
    })
    // TODO: Make this accept a 'contains' word so you can search for text containing also
    .then('I have settings like yaml\n$yaml', function (data) {
      // TODO: Inject this
      const settings = window.localStorage;
      // TODO: this and the setup should probably use consul:
      // as we are talking about 'settings' here not localStorage
      // so the prefix should be hidden
      Object.keys(data).forEach(function (prop) {
        const actual = settings.getItem(prop);
        const expected = data[prop];
        assert.strictEqual(actual, expected, `Expected settings to be ${expected} was ${actual}`);
      });
    })
    .then('the url should match $url', function (url) {
      const currentUrl = currentURL() || '';

      const matches = !!currentUrl.match(url);

      assert.ok(matches, `Expected currentURL to match the provided regex: ${url}`);
    })
    .then('the url should be $url', function (url) {
      // TODO: nice! $url should be wrapped in ""
      if (url === "''") {
        url = '';
      }
      const current = currentURL() || '';
      assert.equal(current, url, `Expected the url to be ${url} was ${current}`);
    })
    .then(['the title should be "$title"'], function (title) {
      assert.equal(document.title, title, `Expected the document.title to equal "${title}"`);
    })
    .then(['the "$selector" input should have the value "$value"'], function (selector, value) {
      const $el = find(selector);
      assert.equal(
        $el.value,
        value,
        `Expected the input at ${selector} to have value ${value}, but it had ${$el.value}`
      );
    });
}
