/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// This files export is executed from 2 places:
// 1. consul-ui/tests/acceptance/steps/steps.js - run during testing
// 2. consul-ui/lib/commands/lib/list.js - run when listing steps via the CLI

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

export default function ({
  assert,
  utils,
  library,
  pages = {},
  helpers = {},
  api = {},
  Inflector = {},
  $ = {},
}) {
  const pluralize = function (str) {
    return Inflector.inflector.pluralize(str);
  };
  const getLastNthRequest = function (getRequests) {
    return function (n, method) {
      let requests = getRequests().slice(0).reverse();
      if (method) {
        requests = requests.filter(function (item) {
          return item.method === method;
        });
      }
      if (n == null) {
        return requests;
      }
      return requests[n];
    };
  };
  const pauseUntil = function (run, message = 'assertion timed out') {
    return new Promise(function (r) {
      let count = 0;
      let resolved = false;
      const retry = function () {
        return Promise.resolve();
      };
      const reject = function () {
        return Promise.reject();
      };
      const resolve = function (str = message) {
        resolved = true;
        assert.ok(resolved, str);
        r();
        return Promise.resolve();
      };
      (function tick() {
        run(resolve, reject, retry).then(function () {
          if (!resolved) {
            setTimeout(function () {
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
  const create = function (number, name, value) {
    // don't return a promise here as
    // I don't need it to wait
    api.server.createList(name, number, value);
  };
  const respondWith = function (url, data) {
    api.server.respondWith(url.split('?')[0], data);
  };
  const setCookie = function (key, value) {
    document.cookie = `${key}=${value}`;
    api.server.setCookie(key, value);
  };

  const reset = function () {
    api.server.clearHistory();
  };

  const clipboard = function () {
    return window.localStorage.getItem('clipboard');
  };
  const currentURL = function () {
    const context = helpers.getContext();
    const locationType = context.owner.lookup('service:env').var('locationType');
    let location = context.owner.lookup(`location:${locationType}`);
    return location.getURLFrom();
  };
  const oidcProvider = function (name, response) {
    const context = helpers.getContext();
    const provider = context.owner.lookup('torii-provider:oidc-with-url');
    provider.popup.open = async function () {
      return response;
    };
  };

  models(library, create, setCookie);
  http(library, respondWith, setCookie, oidcProvider);
  visit(library, pages, utils.setCurrentPage, reset);
  click(library, utils.find, helpers.click);
  form(library, utils.find, helpers.fillIn, helpers.triggerKeyEvent, utils.getCurrentPage);
  debug(library, assert, currentURL);
  assertHttp(library, assert, lastNthRequest);
  assertModel(library, assert, utils.find, utils.getCurrentPage, pauseUntil, pluralize);
  assertPage(library, assert, utils.find, utils.getCurrentPage, $);
  assertDom(library, assert, pauseUntil, helpers.find, currentURL, clipboard);
  assertForm(library, assert, utils.find, utils.getCurrentPage);

  return library.given(["I'm using a legacy token"], function (number, model, data) {
    window.localStorage['consul:token'] = JSON.stringify({
      Namespace: 'default',
      AccessorID: null,
      SecretID: 'id',
    });
  });
}
