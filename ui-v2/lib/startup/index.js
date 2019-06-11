/* eslint-env node */
'use strict';
module.exports = {
  name: 'startup',
  contentFor: function(type, config) {
    const vars = {
      appName: config.modulePrefix,
      environment: config.environment,
      rootURL: config.environment === 'production' ? '{{.ContentPath}}' : config.rootURL,
      config: config,
    };
    switch (type) {
      case 'head':
        return require('./templates/head.html.js')(vars);
      case 'body':
        return require('./templates/body.html.js')(vars);
      case 'root-class':
        return 'ember-loading';
    }
  },
};
