/* eslint no-console: "off", no-control-regex: "off" */
import YAML from 'js-yaml';
import Inflector from 'ember-inflector';
import utils from '@ember/test-helpers';

import Yadda from 'yadda';
import pages from 'consul-ui/tests/pages';
import api from 'consul-ui/tests/helpers/api';

import steps from 'consul-ui/tests/steps';

const pluralize = function(str) {
  return Inflector.inflector.pluralize(str);
};
export default function(assert) {
  const library = Yadda.localisation.English.library(
    new Yadda.Dictionary()
      .define('json', /([^\u0000]*)/, function(val, cb) {
        cb(null, JSON.parse(val));
      })
      .define('yaml', /([^\u0000]*)/, function(val, cb) {
        cb(null, YAML.safeLoad(val));
      })
      .define('model', /(\w+)/, function(model, cb) {
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
          case 'sessions':
            model = 'session';
            break;
          case 'intentions':
            model = 'intention';
            break;
        }
        cb(null, model);
      })
      .define('number', /(\d+)/, Yadda.converters.integer)
  );
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
  const getLastNthRequest = function(arr) {
    return function(n, method) {
      let requests = arr.slice(0).reverse();
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
  return steps(assert, library, pages, {
    pluralize: pluralize,
    triggerKeyEvent: utils.triggerKeyEvent,
    currentURL: utils.currentURL,
    click: utils.click,
    fillIn: utils.fillIn,
    find: utils.find,
    lastNthRequest: getLastNthRequest(api.server.history),
    respondWith: respondWith,
    create: create,
    set: setCookie,
  });
}
