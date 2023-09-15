/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-env node */
const chalk = require('chalk');

module.exports = Object.assign(require('ember-source/blueprints/route/index.js'), {
  afterInstall: function (options) {
    updateRouter.call(this, 'add', options);
  },

  afterUninstall: function (options) {
    updateRouter.call(this, 'remove', options);
  },
});

function updateRouter(action, options) {
  var entity = options.entity;
  var actionColorMap = {
    add: 'green',
    remove: 'red',
  };
  var color = actionColorMap[action] || 'gray';

  if (this.shouldTouchRouter(entity.name, options)) {
    this.ui.writeLine(
      `we don't currently update the router for you, please edit ${findRouter(options).join('/')}`
    );
    this._writeStatusToUI(chalk[color], action + ' route', entity.name);
  }
}

function findRouter(options) {
  var routerPathParts = [options.project.root];

  if (options.dummy && options.project.isEmberCLIAddon()) {
    routerPathParts = routerPathParts.concat(['tests', 'dummy', 'app', 'router.js']);
  } else {
    routerPathParts = routerPathParts.concat(['app', 'router.js']);
  }

  return routerPathParts;
}
