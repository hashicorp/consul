export default function(scenario, assert, find, currentPage, pauseUntil, pluralize) {
  scenario
    .then('pause until I see $number $model model[s]?', function(num, model) {
      return pauseUntil(function(resolve) {
        const len = currentPage()[pluralize(model)].filter(function(item) {
          return item.isVisible;
        }).length;
        if (len === num) {
          assert.equal(len, num, `Expected ${num} ${model}s, saw ${len}`);
          resolve();
        }
      });
    })
    .then(['I see $num $model model[s]?'], function(num, model) {
      const len = currentPage()[pluralize(model)].filter(function(item) {
        return item.isVisible;
      }).length;

      assert.equal(len, num, `Expected ${num} ${pluralize(model)}, saw ${len}`);
    })
    .then(['I see $num $model model[s]? on the $component component'], function(
      num,
      model,
      component
    ) {
      const obj = find(component);
      const len = obj[pluralize(model)].filter(function(item) {
        return item.isVisible;
      }).length;

      assert.equal(len, num, `Expected ${num} ${pluralize(model)}, saw ${len}`);
    })
    // TODO: I${ dont } see
    .then([`I see $num $model model[s]? with the $property "$value"`], function(
      // negate,
      num,
      model,
      property,
      value
    ) {
      const len = currentPage()[pluralize(model)].filter(function(item) {
        return item.isVisible && item[property] == value;
      }).length;
      assert.equal(
        len,
        num,
        `Expected ${num} ${pluralize(model)} with ${property} set to "${value}", saw ${len}`
      );
    });
}
