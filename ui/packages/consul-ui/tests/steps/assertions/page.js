/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/*eslint no-console: "off", ember/no-jquery: "off", ember/no-global-jquery: "off"*/

const elementNotFound = 'Element not found';
// this error comes from our pageObject `find `function
const pageObjectNotFound = 'PageObject not found';
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
const isExpectedError = function (e) {
  return [pageObjectNotFound, elementNotFound, cannotDestructure, cannotReadContext].some((item) =>
    e.message.startsWith(item)
  );
};
const dont = `( don't| shouldn't| can't)?`;
export default function (scenario, assert, find, currentPage, $) {
  scenario
    .then(
      [`I${dont} $verb the $pageObject object`],
      async function (negative, verb, element, next) {
        let res = element[verb];
        if (typeof res === 'function') {
          res = res.call(element);
        }
        assert[negative ? 'notOk' : 'ok'](res, this.step);

        await res;

        setTimeout(() => next());
      }
    )
    .then(
      [
        `I${dont} $verb the $pageObject object with value "$value"`,
        `I${dont} $verb the $pageObject object from $yaml`,
      ],
      function (negative, verb, element, data, next) {
        assert[negative ? 'notOk' : 'ok'](element[verb](data));
        setTimeout(() => next());
      }
    )
    .then(`the $pageObject object is(n't)? $state`, function (element, negative, state, next) {
      assert[negative ? 'notOk' : 'ok'](element[state]);
      setTimeout(() => next());
    })
    .then(`I${dont} see $num of the $pageObject objects`, function (negative, num, element, next) {
      assert[negative ? 'notEqual' : 'equal'](
        element.length,
        num,
        `Expected to${negative ? ' not' : ''} see ${num} ${element.key}`
      );
      setTimeout(() => next());
    })
    .then(['I see $num of the $component object'], function (num, component) {
      assert.equal(
        currentPage()[component].length,
        num,
        `Expected to see ${num} items in the ${component} object`
      );
    })
    .then(
      'I see $property on the $component like yaml\n$yaml',
      function (property, component, yaml) {
        const _component = currentPage()[component];
        const iterator = new Array(_component.length).fill(true);
        // this will catch if we get aren't managing to select a component
        assert.ok(iterator.length > 0);
        iterator.forEach(function (item, i, arr) {
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
      }
    )
    .then(
      'I see $property on the $component vertically like yaml\n$yaml',
      function (property, component, yaml) {
        const _component = find(component);
        const iterator = new Array(_component.length).fill(true);
        assert.ok(iterator.length > 0);

        const items = _component.toArray().sort((a, b) => {
          return (
            $(a.scope).get(0).getBoundingClientRect().top -
            $(b.scope).get(0).getBoundingClientRect().top
          );
        });

        iterator.forEach(function (item, i, arr) {
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
      }
    )
    .then(
      [`I${dont} see $property`, `I${dont} see $property on the $component`],
      function (negative, property, component) {
        const isNegative = typeof negative !== 'undefined';
        let message = `Expected to${isNegative ? ' not' : ''} see ${property}`;
        let target;
        try {
          if (typeof component === 'string') {
            property = `${component}.${property}`;
            message = `${message} on ${component}`;
          }
          target = find(property);
        } catch (e) {
          if (isNegative) {
            if (isExpectedError(e)) {
              assert.ok(true, message);
              return Promise.resolve();
            } else {
              console.error(e);
              throw e;
            }
          } else {
            console.error(e);
            throw e;
          }
        }
        if (typeof target === 'function') {
          if (isNegative) {
            assert.throws(
              function () {
                target();
              },
              function (e) {
                return isExpectedError(e);
              },
              message
            );
            return Promise.resolve();
          } else {
            try {
              target = target();
            } catch (e) {
              console.error(e);
              throw new Error(`The '${property}' page object doesn't exist`);
            }
          }
        }
        assert[isNegative ? 'notOk' : 'ok'](target, message);

        // always return promise and handle the fact that `target` could be async
        return Promise.resolve().then(() => target);
      }
    )
    .then(
      [
        `I see $property on the $component (contains|like) "$value"`,
        `I see $property on the $component (contains|like) '$value'`,
      ],
      function (property, component, containsLike, value) {
        let target;

        if (typeof component === 'string') {
          property = `${component}.${property}`;
        }
        target = find(property);

        if (containsLike === 'like') {
          assert.equal(
            target,
            value,
            `Expected to see ${property} on ${component} as ${value}, was ${target}`
          );
        } else {
          assert.ok(
            target.indexOf(value) !== -1,
            `Expected to see ${property} on ${component} within ${value}, was ${target}`
          );
        }
      }
    )
    .then(['I see $property like "$value"'], function (property, value) {
      const target = currentPage()[property];
      assert.equal(target, value, `Expected to see ${property} as ${value}, was ${target}`);
    });
}
