export default function (scenario, assert, find, currentPage, pauseUntil, pluralize) {
  function getModelItems(model, component) {
    let obj;
    if (component) {
      obj = find(component);
    } else {
      obj = currentPage();
    }

    let found = obj[pluralize(model)];

    if (typeof found === 'function') {
      found = found();
    }

    return found;
  }

  scenario
    .then('pause until I see $number $model model[s]?', function (num, model) {
      return pauseUntil(function (resolve, reject, retry) {
        const len = getModelItems(model).filter(function (item) {
          return item.isVisible;
        }).length;
        if (len === num) {
          return resolve();
        }
        return retry();
      }, `Expected ${num} ${model}s`);
    })
    .then(
      'pause until I see $number $model model[s]? on the $component component',
      function (num, model, component) {
        return pauseUntil(function (resolve, reject, retry) {
          const len = getModelItems(model, component).filter(function (item) {
            return item.isVisible;
          }).length;
          if (len === num) {
            return resolve();
          }
          return retry();
        }, `Expected ${num} ${model}s`);
      }
    )
    .then(['I see $num $model model[s]?'], function (num, model) {
      const len = getModelItems(model).filter(function (item) {
        return item.isVisible;
      }).length;
      assert.equal(len, num, `Expected ${num} ${pluralize(model)}, saw ${len}`);
    })
    .then(
      ['I see $num $model model[s]? on the $component component'],
      function (num, model, component) {
        const len = getModelItems(model, component).filter(function (item) {
          return item.isVisible;
        }).length;

        assert.equal(len, num, `Expected ${num} ${pluralize(model)}, saw ${len}`);
      }
    )
    .then(
      [`I see $num $model model[s]? with the $property "$value"`],
      function (
        // negate,
        num,
        model,
        property,
        value
      ) {
        const len = getModelItems(model).filter(function (item) {
          if (item.isVisible) {
            let prop = item[property];
            // cope with pageObjects that can have a multiple: true
            if (!Array.isArray(prop)) {
              prop = [prop];
            }
            return prop.includes(value);
          }
          return false;
        }).length;
        assert.equal(
          len,
          num,
          `Expected ${num} ${pluralize(model)} with ${property} set to "${value}", saw ${len}`
        );
      }
    );
}
