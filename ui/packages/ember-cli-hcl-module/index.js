'use strict';
const Filter = require('broccoli-persistent-filter');
const hcl = require('micro-hcl');

class HCLModule extends Filter {
  constructor(inputNode, options = {}) {
    super(
      inputNode,
      {
        persist: options.persist === true
      }
    );
    this.extensions = ['hcl'];
    this.targetExtension = 'hcl.js';
  }
  processString(string) {
    const json = JSON.stringify(hcl.parse(string), null, 4);
    return "export default " + json + ";";
  }
}

const hclModule = function(inputTree, options) {
  return new HCLModule(inputTree, options);
}
module.exports = {
  name: require('./package').name,
  treeForApp: function() {
    return hclModule(this.app.trees.app);
  },
  treeForTestSupport: function() {
    return hclModule(this.app.trees.tests);
  }
};
