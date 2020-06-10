import pages from 'consul-ui/tests/pages';
import Inflector from 'ember-inflector';
import utils from '@ember/test-helpers';

import api from 'consul-ui/tests/helpers/api';

import models from './steps/doubles/model';
import http from './steps/doubles/http';
import visit from './steps/interactions/visit';
import click from './steps/interactions/click';
import form from './steps/interactions/form';
import debug from './steps/debug/index';
import assertHttp from './steps/assertions/http';
import assertModel from './steps/assertions/model';
import assertPage from './steps/assertions/page';
import assertDom from './steps/assertions/dom';
import assertForm from './steps/assertions/form';

// const dont = `( don't| shouldn't| can't)?`;

const pluralize = function(str) {
  return Inflector.inflector.pluralize(str);
};
const getLastNthRequest = function(getRequests) {
  return function(n, method) {
    let requests = getRequests()
      .slice(0)
      .reverse();
    if (method) {
      requests = requests.filter(function(item) {
        return item.method === method;
      });
    }
    if (n == null) {
      return requests;
    }
    return requests[n];
  };
};
const mb = function(path) {
  return function(obj) {
    return (
      path.map(function(prop) {
        obj = obj || {};
        if (isNaN(parseInt(prop))) {
          return (obj = obj[prop]);
        } else {
          return (obj = obj.objectAt(parseInt(prop)));
        }
      }) && obj
    );
  };
};
export default function(assert, library) {
  const pauseUntil = function(run, message = 'assertion timed out') {
    return new Promise(function(r) {
      let count = 0;
      let resolved = false;
      const retry = function() {
        return Promise.resolve();
      };
      const reject = function() {
        return Promise.reject();
      };
      const resolve = function(str = message) {
        resolved = true;
        assert.ok(resolved, str);
        r();
        return Promise.resolve();
      };
      (function tick() {
        run(resolve, reject, retry).then(function() {
          if (!resolved) {
            setTimeout(function() {
              if (++count >= 50) {
                assert.ok(false, message);
                reject();
                return;
              }
              tick();
            }, 100);
          }
        });
      })();
    });
  };
  const lastNthRequest = getLastNthRequest(() => api.server.history);
  const create = function(number, name, value) {
    // don't return a promise here as
    // I don't need it to wait
    api.server.createList(name, number, value);
  };
  const respondWith = function(url, data) {
    api.server.respondWith(url.split('?')[0], data);
  };
  const setCookie = function(key, value) {
    api.server.setCookie(key, value);
  };
  let currentPage;
  const getCurrentPage = function() {
    return currentPage;
  };
  const setCurrentPage = function(page) {
    api.server.clearHistory();
    currentPage = page;
    return page;
  };

  const find = function(path) {
    const page = getCurrentPage();
    const parts = path.split('.');
    const last = parts.pop();
    let obj;
    let parent = mb(parts)(page) || page;
    if (typeof parent.objectAt === 'function') {
      parent = parent.objectAt(0);
    }
    obj = parent[last];
    if (typeof obj === 'undefined') {
      throw new Error(`PageObject not found: The '${path}' object doesn't exist`);
    }
    if (typeof obj === 'function') {
      obj = obj.bind(parent);
    }
    return obj;
  };
  const clipboard = function() {
    return window.localStorage.getItem('clipboard');
  };
  models(library, create);
  http(library, respondWith, setCookie);
  visit(library, pages, setCurrentPage);
  click(library, find, utils.click);
  form(library, find, utils.fillIn, utils.triggerKeyEvent, getCurrentPage);
  debug(library, assert, utils.currentURL);
  assertHttp(library, assert, lastNthRequest);
  assertModel(library, assert, find, getCurrentPage, pauseUntil, pluralize);
  assertPage(library, assert, find, getCurrentPage);
  assertDom(library, assert, pauseUntil, utils.find, utils.currentURL, clipboard);
  assertForm(library, assert, find, getCurrentPage);

  return library.given(["I'm using a legacy token"], function(number, model, data) {
    window.localStorage['consul:token'] = JSON.stringify({
      Namespace: 'default',
      AccessorID: null,
      SecretID: 'id',
    });
  });
}
