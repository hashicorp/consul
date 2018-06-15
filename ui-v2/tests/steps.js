/* eslint no-console: "off" */
import yadda from './helpers/yadda';
import { currentURL, click, triggerKeyEvent, fillIn, find } from '@ember/test-helpers';
import getDictionary from '@hashicorp/ember-cli-api-double/dictionary';
import pages from 'consul-ui/tests/pages';
import api from 'consul-ui/tests/helpers/api';

const create = function(number, name, value) {
  // don't return a promise here as
  // I don't need it to wait
  api.server.createList(name, number, value);
};
var currentPage;
export default function(assert) {
  return (
    yadda.localisation.English.library(
      getDictionary(function(model, cb) {
        switch (model) {
          case 'datacenter':
          case 'datacenters':
          case 'dcs':
            model = 'dc';
            break;
          case 'services':
            model = 'service';
            break;
          case 'nodes':
            model = 'node';
            break;
          case 'kvs':
            model = 'kv';
            break;
          case 'acls':
            model = 'acl';
            break;
          case 'intentions':
            model = 'intention';
            break;
        }
        cb(null, model);
      }, yadda)
    )
      // doubles
      .given(['$number $model model', '$number $model models'], function(number, model) {
        return create(number, model);
      })
      .given(['$number $model model with the value "$value"'], function(number, model, value) {
        return create(number, model, value);
      })
      .given(
        ['$number $model model[s]? from yaml\n$yaml', '$number $model model from json\n$json'],
        function(number, model, data) {
          return create(number, model, data);
        }
      )
      // interactions
      .when('I visit the $name page', function(name) {
        currentPage = pages[name];
        return currentPage.visit();
      })
      .when('I visit the $name page for the "$id" $model', function(name, id, model) {
        currentPage = pages[name];
        return currentPage.visit({
          [model]: id,
        });
      })
      .when(
        ['I visit the $name page for yaml\n$yaml', 'I visit the $name page for json\n$json'],
        function(name, data) {
          currentPage = pages[name];
          // TODO: Consider putting an assertion here for testing the current url
          // do I absolutely definitely need that all the time?
          return pages[name].visit(data);
        }
      )
      .when('I click "$selector"', function(selector) {
        return click(selector);
      })
      .when('I click $prop on the $component', function(prop, component) {
        // Collection
        var obj;
        if (typeof currentPage[component].objectAt === 'function') {
          obj = currentPage[component].objectAt(0);
        } else {
          obj = currentPage[component];
        }
        const func = obj[prop].bind(obj);
        try {
          return func();
        } catch (e) {
          console.error(e);
          throw new Error(`The '${prop}' property on the '${component}' page object doesn't exist`);
        }
      })
      .when('I submit', function(selector) {
        return currentPage.submit();
      })
      .then('I fill in "$name" with "$value"', function(name, value) {
        return currentPage.fillIn(name, value);
      })
      .then(['I fill in with yaml\n$yaml', 'I fill in with json\n$json'], function(data) {
        return Object.keys(data).reduce(function(prev, item, i, arr) {
          return prev.fillIn(item, data[item]);
        }, currentPage);
      })
      .then(['I type "$text" into "$selector"'], function(text, selector) {
        return fillIn(selector, text);
      })
      .then(['I type with yaml\n$yaml'], function(data) {
        const keys = Object.keys(data);
        return keys
          .reduce(function(prev, item, i, arr) {
            return prev.fillIn(item, data[item]);
          }, currentPage)
          .then(function() {
            return Promise.all(
              keys.map(function(item) {
                return triggerKeyEvent(`[name="${item}"]`, 'keyup', 83); // TODO: This is 's', be more generic
              })
            );
          });
      })
      // debugging helpers
      .then('print the current url', function(url) {
        console.log(currentURL());
        return Promise.resolve();
      })
      .then('log the "$text"', function(text) {
        console.log(text);
        return Promise.resolve();
      })
      .then('pause for $milliseconds', function(milliseconds) {
        return new Promise(function(resolve) {
          setTimeout(resolve, milliseconds);
        });
      })
      // assertions
      .then('a $method request is made to "$url" with the body from yaml\n$yaml', function(
        method,
        url,
        data
      ) {
        const request = api.server.history[api.server.history.length - 2];
        assert.equal(
          request.method,
          method,
          `Expected the request method to be ${method}, was ${request.method}`
        );
        assert.equal(request.url, url, `Expected the request url to be ${url}, was ${request.url}`);
        const body = JSON.parse(request.requestBody);
        Object.keys(data).forEach(function(key, i, arr) {
          assert.equal(
            body[key],
            data[key],
            `Expected the payload to contain ${key} to equal ${body[key]}, ${key} was ${data[key]}`
          );
        });
      })
      .then('a $method request is made to "$url" with the body "$body"', function(
        method,
        url,
        data
      ) {
        const request = api.server.history[api.server.history.length - 2];
        assert.equal(
          request.method,
          method,
          `Expected the request method to be ${method}, was ${request.method}`
        );
        assert.equal(request.url, url, `Expected the request url to be ${url}, was ${request.url}`);
        const body = request.requestBody;
        assert.equal(
          body,
          data,
          `Expected the request body to be ${body}, was ${request.requestBody}`
        );
      })
      .then('a $method request is made to "$url"', function(method, url) {
        const request = api.server.history[api.server.history.length - 2];
        assert.equal(
          request.method,
          method,
          `Expected the request method to be ${method}, was ${request.method}`
        );
        assert.equal(request.url, url, `Expected the request url to be ${url}, was ${request.url}`);
      })
      .then('the url should be $url', function(url) {
        const current = currentURL();
        assert.equal(current, url, `Expected the url to be ${url} was ${current}`);
      })
      .then(['I see $num $model', 'I see $num $model model', 'I see $num $model models'], function(
        num,
        model
      ) {
        const len = currentPage[`${model}s`].filter(function(item) {
          return item.isVisible;
        }).length;

        assert.equal(len, num, `Expected ${num} ${model}s, saw ${len}`);
      })
      .then(['I see $num $model model with the $property "$value"'], function(
        num,
        model,
        property,
        value
      ) {
        const len = currentPage[`${model}s`].filter(function(item) {
          return item.isVisible && item[property] == value;
        }).length;
        assert.equal(
          len,
          num,
          `Expected ${num} ${model}s with ${property} set to "${value}", saw ${len}`
        );
      })
      .then('I see $property on the $component like yaml\n$yaml', function(
        property,
        component,
        yaml
      ) {
        const _component = currentPage[component];
        const iterator = new Array(_component.length).fill(true);
        iterator.forEach(function(item, i, arr) {
          const actual = _component.objectAt(i)[property];
          const expected = yaml[i];
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
        if (typeof currentPage[component].objectAt === 'function') {
          obj = currentPage[component].objectAt(0);
        } else {
          obj = currentPage[component];
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
      .then(["I don't see $property on the $component"], function(property, component) {
        // Collection
        var obj;
        if (typeof currentPage[component].objectAt === 'function') {
          obj = currentPage[component].objectAt(0);
        } else {
          obj = currentPage[component];
        }
        const func = obj[property].bind(obj);
        assert.throws(
          function() {
            func();
          },
          function(e) {
            return e.toString().indexOf('Element not found') !== -1;
          },
          `Expected to not see ${property} on ${component}`
        );
      })
      .then(['I see $property'], function(property, component) {
        assert.ok(currentPage[property], `Expected to see ${property}`);
      })
      .then(['I see "$text" in "$selector"'], function(text, selector) {
        assert.ok(
          find(selector).textContent.indexOf(text) !== -1,
          `Expected to see ${text} in ${selector}`
        );
      })
      .then('ok', function() {
        assert.ok(true);
      })
  );
}
