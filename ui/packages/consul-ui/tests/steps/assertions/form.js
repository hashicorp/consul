/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (scenario, assert, find, currentPage) {
  scenario.then('I see the $property form with yaml\n$yaml', function (property, data) {
    let obj;
    try {
      obj = find(property);
    } catch (e) {
      obj = currentPage();
    }
    return Object.keys(data).reduce(function (prev, item, i, arr) {
      const name = `${obj.prefix || property}[${item}]`;
      const $el =
        document.querySelector(`[name="${name}"]`) ||
        document.querySelector(`[aria-label="${name}"]`);
      const actual = $el.value || $el.textContent;
      const expected = data[item];
      assert.strictEqual(actual, expected, `Expected settings to be ${expected} was ${actual}`);
    }, obj);
  });
}
