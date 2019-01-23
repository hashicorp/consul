/* eslint no-console: "off" */
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
export default function(assert, library, api, pages, utils, pluralize) {
  const create = function(number, name, value) {
    // don't return a promise here as
    // I don't need it to wait
    api.server.createList(name, number, value);
  };
  var currentPage;
  const getCurrentPage = function() {
    return currentPage;
  };
  const setCurrentPage = function(page) {
    currentPage = page;
    return page;
  };

  models(library, create);
  http(library, api);
  visit(library, pages, setCurrentPage);
  click(library, utils.click, getCurrentPage);
  form(library, utils.fillIn, utils.triggerKeyEvent, getCurrentPage);
  debug(library, assert, utils.currentURL);
  assertHttp(library, assert, api);
  assertModel(library, assert, getCurrentPage, pluralize);
  assertPage(library, assert, getCurrentPage);
  assertDom(library, assert, utils.find, utils.currentURL);

  return library.given(["I'm using a legacy token"], function(number, model, data) {
    window.localStorage['consul:token'] = JSON.stringify({ AccessorID: null, SecretID: 'id' });
  });
}
