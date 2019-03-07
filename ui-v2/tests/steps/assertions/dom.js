export default function(scenario, assert, find, currentURL) {
  scenario
    .then(['I see the text "$text" in "$selector"'], function(text, selector) {
      assert.ok(
        find(selector).textContent.indexOf(text) !== -1,
        `Expected to see "${text}" in "${selector}"`
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
      // because `find` doesn't work, guessing its sandboxed to ember's container
      assert.ok(
        document.querySelector(selector).classList.contains(cls),
        `Expected [class] to contain ${cls} on ${selector}`
      );
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
      // TODO: nice! $url should be wrapped in ""
      if (url === "''") {
        url = '';
      }
      const current = currentURL() || '';
      assert.equal(current, url, `Expected the url to be ${url} was ${current}`);
    });
}
