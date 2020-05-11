/* eslint no-console: "off" */
import $ from '-jquery';

const notFound = 'Element not found';
const cannotDestructure = "Cannot destructure property 'context'";
const cannotReadContext = "Cannot read property 'context' of undefined";
// checking for existence of pageObjects is pretty difficult
// errors are thrown but we should check to make sure its the error that we
// want and not another real error
// to make things more difficult depending on how you reference the pageObject
// an error with a different message is thrown for example:

// pageObject[thing]() will give you a Element not found error

// whereas:

// const obj = pageObject[thing]; obj() will give you a 'cannot destructure error'
// and in CI it will give you a 'cannot read property' error

// the difference in CI could be a difference due to headless vs headed browser
// or difference in Chrome/browser versions

// ideally we wouldn't be checking on error messages at all, but we want to make sure
// that real errors are picked up by the tests, so if this gets unmanageable at any point
// look at checking for the instance of e being TypeError or similar
const isExpectedError = function(e) {
  return [notFound, cannotDestructure, cannotReadContext].some(item => e.message.startsWith(item));
};

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
    .then('I see $property on the $component vertically like yaml\n$yaml', function(
      property,
      component,
      yaml
    ) {
      const _component = currentPage()[component];
      const iterator = new Array(_component.length).fill(true);
      assert.ok(iterator.length > 0);

      const items = _component.toArray().sort((a, b) => {
        return (
          $(a.scope)
            .get(0)
            .getBoundingClientRect().top -
          $(b.scope)
            .get(0)
            .getBoundingClientRect().top
        );
      });

      iterator.forEach(function(item, i, arr) {
        const actual = typeof items[i][property] === 'undefined' ? null : items[i][property];

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
      const message = `Expected to not see ${property} on ${component}`;
      // Cope with collections
      let obj;
      if (typeof currentPage()[component].objectAt === 'function') {
        obj = currentPage()[component].objectAt(0);
      } else {
        obj = currentPage()[component];
      }
      let prop;
      try {
        prop = obj[property];
      } catch (e) {
        if (isExpectedError(e)) {
          assert.ok(true, message);
        } else {
          throw e;
        }
      }
      if (typeof prop === 'function') {
        assert.throws(
          function() {
            prop();
          },
          function(e) {
            return isExpectedError(e);
          },
          message
        );
      } else {
        assert.notOk(prop);
      }
    })
    .then(["I don't see $property"], function(property) {
      const message = `Expected to not see ${property}`;
      let prop;
      try {
        prop = currentPage()[property];
      } catch (e) {
        if (isExpectedError(e)) {
          assert.ok(true, message);
        } else {
          throw e;
        }
      }
      if (typeof prop === 'function') {
        assert.throws(
          function() {
            prop();
          },
          function(e) {
            return isExpectedError(e);
          },
          message
        );
      } else {
        assert.notOk(prop);
      }
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
