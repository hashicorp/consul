export default function(scenario, assert, pauseUntil, find, currentURL, clipboard) {
  scenario
    .then(
      [
        'I see the text "$text" in "$selector"',
        'pause until I see the text "$text" in "$selector"',
      ],
      function(text, selector) {
        return pauseUntil(function(resolve, reject) {
          const $el = find(selector);
          if ($el) {
            const hasText = $el.textContent.indexOf(text) !== -1;
            if (hasText) {
              assert.ok(hasText, `Expected to see "${text}" in "${selector}"`);
              resolve();
            } else {
              reject(`Expected to see "${text}" in "${selector}"`);
            }
          }
          return Promise.resolve();
        });
      }
    )
    .then(['I copied "$text"'], function(text) {
      const copied = clipboard();
      assert.ok(
        copied.indexOf(text) !== -1,
        `Expected to see "${text}" in the clipboard, was "${copied}"`
      );
    })
    .then(['I see the exact text "$text" in "$selector"'], function(text, selector) {
      assert.ok(
        find(selector).textContent.trim() === text,
        `Expected to see the exact "${text}" in "${selector}"`
      );
    })
    // TODO: Think of better language
    // TODO: These should be mergeable
    .then(['"$selector" has the "$class" class'], function(selector, cls) {
      return pauseUntil(function(resolve, reject) {
        // because `find` doesn't work, guessing its sandboxed to ember's container
        const $el = document.querySelector(selector);
        if ($el) {
          if ($el.classList.contains(cls)) {
            assert.ok(true, `Expected [class] to contain ${cls} on ${selector}`);
            resolve();
          } else {
            reject(`Expected [class] to contain ${cls} on ${selector}`);
          }
        }
        return Promise.resolve();
      });
    })
    .then([`I don't see the "$selector" element`], function(selector) {
      assert.equal(document.querySelector(selector), null, `Expected not to see ${selector}`);
    })
    .then(['"$selector" doesn\'t have the "$class" class'], function(selector, cls) {
      assert.ok(
        !document.querySelector(selector).classList.contains(cls),
        `Expected [class] not to contain ${cls} on ${selector}`
      );
    })
    // TODO: Make this accept a 'contains' word so you can search for text containing also
    .then('I have settings like yaml\n$yaml', function(data) {
      // TODO: Inject this
      const settings = window.localStorage;
      // TODO: this and the setup should probably use consul:
      // as we are talking about 'settings' here not localStorage
      // so the prefix should be hidden
      Object.keys(data).forEach(function(prop) {
        const actual = settings.getItem(prop);
        const expected = data[prop];
        assert.strictEqual(actual, expected, `Expected settings to be ${expected} was ${actual}`);
      });
    })
    .then('the url should be $url', function(url) {
      return pauseUntil(function(resolve, reject) {
        // TODO: nice! $url should be wrapped in ""
        if (url === "''") {
          url = '';
        }
        const current = currentURL() || '';
        if (current === url) {
          assert.equal(current, url, `Expected the url to be ${url} was ${current}`);
          resolve();
        }
        return Promise.resolve();
      });
    })
    .then(['the title should be "$title"'], function(title) {
      assert.equal(document.title, title, `Expected the document.title to equal "${title}"`);
    });
}
