/* eslint-env node */
'use strict';

const path = require('path');
const Funnel = require('broccoli-funnel');
module.exports = {
  name: 'iris',

  isDevelopingAddon() {
    return true;
  },
  included: function(app) {
    this._super.included.apply(this, arguments);
    while (typeof app.import !== 'function' && app.app) {
      app = app.app;
    }

    this.irisPath = path.dirname(require.resolve('@hashicorp/iris'));
    return app;
  },

  treeForStyles: function() {
    return new Funnel(this.irisPath, {
      srcDir: '/',
      include: ['**/*.scss'],
      destDir: 'app/styles/@hashicorp/iris',
      annotation: 'Funnel (iris)',
    });
  },
};
