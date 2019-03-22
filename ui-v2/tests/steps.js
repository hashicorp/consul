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

// const dont = `( don't| shouldn't| can't)?`;

export default function(assert, library, pages, utils) {
  var currentPage;
  const getCurrentPage = function() {
    return currentPage;
  };
  const setCurrentPage = function(page) {
    currentPage = page;
    return page;
  };

  const pauseUntil = function(cb) {
    return new Promise(function(resolve, reject) {
      let count = 0;
      const interval = setInterval(function() {
        if (++count >= 50) {
          clearInterval(interval);
          assert.ok(false);
          reject();
        }
        cb(function() {
          clearInterval(interval);
          resolve();
        });
      }, 100);
    });
  };
  models(library, utils.create);
  http(library, utils.respondWith, utils.set);
  visit(library, pages, setCurrentPage);
  click(library, utils.click, getCurrentPage);
  form(library, utils.fillIn, utils.triggerKeyEvent, getCurrentPage);
  debug(library, assert, utils.currentURL);
  assertHttp(library, assert, utils.lastNthRequest);
  assertModel(library, assert, getCurrentPage, pauseUntil, utils.pluralize);
  assertPage(library, assert, getCurrentPage);
  assertDom(library, assert, pauseUntil, utils.find, utils.currentURL);

  return library.given(["I'm using a legacy token"], function(number, model, data) {
    window.localStorage['consul:token'] = JSON.stringify({ AccessorID: null, SecretID: 'id' });
  });
}
