'use strict';

const mergeTrees = require('broccoli-merge-trees');
const writeFile = require('broccoli-file-creator');

const readdir = require('recursive-readdir-sync');
const read = require('fs').readFileSync;

module.exports = {
  name: require('./package.json').name,
  isDevelopingAddon: function() {
    return true;
  },
  contentFor: function(type, config) {
    const name = this.name;
    const addon = config[name] || {enabled: false};
    if(addon.enabled) {
      const cwd = process.cwd();
      switch (type) {
        case 'body':
          const templates = [];
          Object.keys(addon.endpoints).forEach(
            function(key) {
              const api = `${addon.endpoints[key]}`;
              const absoluteAPI = api;
              readdir(absoluteAPI).map(
                function(item, i, arr) {
                  const url = `${key}${item.replace(api, '')}`;
                  templates.push(`<script type="text/javascript+template" data-url="${url}">${read(item)}</script>`);
                }
              );
            }
          );
          return templates.join('');
      }
    }
  },
  treeForApp: function(appTree) {
    const config = this.app.project.config(this.app.env) || {};
    const addon = config[this.name] || {enabled: false};
    // don't include anything if we aren't enabled
    if (!addon.enabled) {
      return;
    }
    return this._super.treeForApp.apply(this, arguments);
  },
  treeFor: function(name) {
    let app;

    // If the addon has the _findHost() method (in ember-cli >= 2.7.0), we'll just
    // use that.
    if (typeof this._findHost === 'function') {
      app = this._findHost();
    } else {
      // Otherwise, we'll use this implementation borrowed from the _findHost()
      // method in ember-cli.
      let current = this;
      do {
        app = current.app || app;
      } while (current.parent.parent && (current = current.parent));
    }

    this.app = app;
    const config = app.project.config(app.env) || {};
    const addon = config[this.name] || {enabled: false};

    // don't include anything if we aren't enabled
    if (!addon.enabled) {
      return;
    }
    // unless we've explicitly set auto-import to false (in order to provide custom functions)
    // include an initializer to initialize the shim http client
    if(addon['auto-import'] !== false && name === 'app') {
      const tree = writeFile('instance-initializers/ember-cli-api-double.js', `
          import apiDouble from 'ember-cli-api-double';
          apiDouble(JSON.parse('${JSON.stringify(addon)}'));
          export default {
            name: 'ember-cli-api-double',
            initialize: function() {}
          };
      `);
      return mergeTrees([
        tree,
        this._super.treeFor.apply(this, arguments)
      ]);
    }
    return this._super.treeFor.apply(this, arguments);
  },
};
