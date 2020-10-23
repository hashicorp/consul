const faker = require('faker');
const YAML = require('js-yaml');
const range = require('array-range');

let template = require('backtick-template');
if(typeof template !== 'function') {
  template = template.default;
}

class Template {
  constructor(template) {
    this._template = template;
  }
  render(vars) {
    return template(this._template, vars);
  }
}
//
const locationFactory = require('./location/factory.js');
const vars = require('./vars/index.js');
const renderer = require('./lib/render');
const finder = require('./lib/find');
const resolver = require('./lib/resolve');

module.exports = function(seed, path, reader, $, resolve) {
  reader = typeof reader === 'undefined' ? read : reader;
  $ = typeof $ === 'undefined' ? window.localStorage : $;
  resolve = typeof resolve === 'undefined' ? function(path) {
    return path[path.length - 1] === '/' ? path.substr(0, path.length - 1) : path;
  } : resolve;

  return function() {
    const mutations = [];
    const mutate = function(request, content, config) {
      try {
        const isJSON = JSON.parse(content);
      } catch(e) {
        return content;
      }
      return JSON.stringify(
        mutations
          .filter(function(item, i, arr) {
            // TODO: Deprecate checking on strings, should always be a callable
            let cb = item.url;
            if (typeof item.url !== 'function') {
              cb = function(actual) {
                return item.url === request.url;
              };
            }
            // TODO: Deprecate passing 2 arguments pass a request object instead
            return cb(request.url, request.method);
          })
          .reduce(function(prev, item, i, arr) {
            return item.mutate(prev, config);
          }, JSON.parse(content))
      );
    };
    const addMutation = function(cb, url) {
      mutations.push({
        url: url,
        mutate: cb,
      });
    };
    const controller = require('./lib/controller')(
      resolver(resolve, path),
      finder(reader, path),
      renderer(
        vars(locationFactory(), range, $),
        Template,
        faker,
        seed
      ),
      YAML,
      mutate
    );
    return {
      serve: controller,
      mutate: addMutation,
    };
  };
};
