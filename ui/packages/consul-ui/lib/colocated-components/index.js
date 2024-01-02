/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/*eslint node/no-extraneous-require: "off"*/
'use strict';

const Funnel = require('broccoli-funnel');
const mergeTrees = require('broccoli-merge-trees');
const writeFile = require('broccoli-file-creator');
const read = require('fs').readFileSync;

module.exports = {
  name: require('./package').name,

  /**
   * Make any CSS available for import within app/components/component-name:
   * @import 'app-name/components/component-name/index.scss'
   */
  treeForStyles: function (tree) {
    let debug = read(`${this.project.root}/app/styles/debug.scss`);
    if (['production', 'test'].includes(process.env.EMBER_ENV)) {
      debug = '';
    }
    return this._super.treeForStyles.apply(this, [
      mergeTrees([
        writeFile(`_debug.scss`, debug),
        new Funnel(`${this.project.root}/app/components`, {
          destDir: `app/components`,
          include: ['**/*.scss'],
        }),
      ]),
    ]);
  },
};
