/*eslint node/no-extraneous-require: "off"*/
'use strict';

const Funnel = require('broccoli-funnel');
const mergeTrees = require('broccoli-merge-trees');

module.exports = {
  name: require('./package').name,

  /**
   * Make any CSS available for import within app/components/component-name:
   * @import 'app-name/components/component-name/index.scss'
   */
  treeForStyles: function(tree) {
    return this._super.treeForStyles.apply(this, [
      mergeTrees(
        (tree || []).concat([
          new Funnel(`${this.project.root}/app/components`, {
            destDir: `app/components`,
            include: ['**/*.scss'],
          }),
        ])
      ),
    ]);
  },
};
