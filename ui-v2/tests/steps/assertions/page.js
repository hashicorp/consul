/* eslint no-console: "off" */
export default function(scenario, assert, find, currentPage) {
  scenario
    .then('I see $property on the $component like yaml\n$yaml', function(
      property,
      component,
      yaml
    ) {
      const _component = currentPage()[component];
      const iterator = new Array(_component.length).fill(true);
      // this will catch if we get aren't managing to select a component
      assert.ok(iterator.length > 0);
      iterator.forEach(function(item, i, arr) {
        const actual =
          typeof _component.objectAt(i)[property] === 'undefined'
            ? null
            : _component.objectAt(i)[property];

        // anything coming from the DOM is going to be text/strings
        // if the yaml has numbers, cast them to strings
        // TODO: This would get problematic for deeper objects
        // will have to look to do this recursively
        const expected = typeof yaml[i] === 'number' ? yaml[i].toString() : yaml[i];

        assert.deepEqual(
          actual,
          expected,
          `Expected to see ${property} on ${component}[${i}] as ${JSON.stringify(
            expected
          )}, was ${JSON.stringify(actual)}`
        );
      });
    })
    .then(['I see $property on the $component'], function(property, component) {
      // TODO: Time to work on repetition
      // Collection
      var obj;
      if (typeof currentPage()[component].objectAt === 'function') {
        obj = currentPage()[component].objectAt(0);
      } else {
        obj = currentPage()[component];
      }
      let _component;
      if (typeof obj === 'function') {
        const func = obj[property].bind(obj);
        try {
          _component = func();
        } catch (e) {
          console.error(e);
          throw new Error(
            `The '${property}' property on the '${component}' page object doesn't exist`
          );
        }
      } else {
        _component = obj;
      }
      assert.ok(_component[property], `Expected to see ${property} on ${component}`);
    })
    .then(['I see $num of the $component object'], function(num, component) {
      assert.equal(
        currentPage()[component].length,
        num,
        `Expected to see ${num} items in the ${component} object`
      );
    })
    .then(["I don't see $property on the $component"], function(property, component) {
      // Collection
      var obj;
      if (typeof currentPage()[component].objectAt === 'function') {
        obj = currentPage()[component].objectAt(0);
      } else {
        obj = currentPage()[component];
      }
      assert.throws(
        function() {
          const func = obj[property].bind(obj);
          func();
        },
        function(e) {
          return e.message.startsWith('Element not found');
        },
        `Expected to not see ${property} on ${component}`
      );
    })
    .then(["I don't see $property"], function(property) {
      assert.throws(
        function() {
          return currentPage()[property]();
        },
        function(e) {
          return e.message.startsWith('Element not found');
        },
        `Expected to not see ${property}`
      );
    })
    .then(['I see $property'], function(property) {
      assert.ok(currentPage()[property], `Expected to see ${property}`);
    })
    .then(
      [
        'I see $property on the $component like "$value"',
        "I see $property on the $component like '$value'",
      ],
      function(property, component, value) {
        let target;
        try {
          if (typeof component === 'string') {
            property = `${component}.${property}`;
          }
          target = find(property);
        } catch (e) {
          throw e;
        }
        assert.equal(
          target,
          value,
          `Expected to see ${property} on ${component} as ${value}, was ${target}`
        );
      }
    )
    .then(['I see $property like "$value"'], function(property, value) {
      const target = currentPage()[property];
      assert.equal(target, value, `Expected to see ${property} as ${value}, was ${target}`);
    });
}
