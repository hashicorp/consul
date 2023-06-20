/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

'use strict';

module.exports = {
  name: require('./package').name,
  getTransform: function () {
    return {
      name: 'custom-element',
      plugin: class {
        transform(ast) {
          this.syntax.traverse(ast, {
            ElementNode: (node) => {
              if (node.tag === 'CustomElement') {
                node.attributes = node.attributes
                  // completely remove these ones, they are not used runtime
                  // element is potentially only temporarily being removed
                  .filter(
                    (item) =>
                      !['element', 'description', 'slots', 'cssparts'].includes(
                        `${item.name.substr(1)}`
                      )
                  )
                  .map((item) => {
                    switch (true) {
                      // these ones are ones where we need to remove the documentation only
                      // the attributes themselves are required at runtime
                      case ['attrs', 'cssprops'].includes(`${item.name.substr(1)}`):
                        item.value.params = item.value.params.map((item) => {
                          // we can't use arr.length here as we don't know
                          // whether someone has used the documentation entry
                          // in the array or not We use the hardcoded `3` for
                          // the moment if that position needs to change per
                          // property we can just add more cases for the
                          // moment
                          item.params = item.params.filter((item, i, arr) => i < 3);
                          return item;
                        });
                        break;
                    }
                    return item;
                  });
              }
            },
          });
          return ast;
        }
      },
      baseDir: function () {
        return __dirname;
      },
      cacheKey: function () {
        return 'custom-element';
      },
    };
  },
  setupPreprocessorRegistry(type, registry) {
    const transform = this.getTransform();
    transform.parallelBabel = {
      requireFile: __filename,
      buildUsing: 'getTransform',
      params: {},
    };
    registry.add('htmlbars-ast-plugin', transform);
  },
};
