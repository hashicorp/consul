export default function(scenario, assert, currentPage, pluralize) {
  scenario
    .then('pause until I see $number $model model[s]?', function(num, model) {
      return new Promise(function(resolve) {
        let count = 0;
        const interval = setInterval(function() {
          if (++count >= 50) {
            clearInterval(interval);
            assert.ok(false);
            resolve();
          }
          const len = currentPage()[pluralize(model)].filter(function(item) {
            return item.isVisible;
          }).length;
          if (len === num) {
            clearInterval(interval);
            assert.equal(len, num, `Expected ${num} ${model}s, saw ${len}`);
            resolve();
          }
        }, 100);
      });
    })
    .then(['I see $num $model model[s]?'], function(num, model) {
      const len = currentPage()[pluralize(model)].filter(function(item) {
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
