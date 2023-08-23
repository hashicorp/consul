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
      const $el = document.querySelector(`[name="${name}"]`);
      const actual = $el.value;
      const expected = data[item];
      assert.strictEqual(actual, expected, `Expected settings to be ${expected} was ${actual}`);
    }, obj);
  });
}
