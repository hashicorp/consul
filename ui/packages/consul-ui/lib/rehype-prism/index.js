// modified from https://github.com/mapbox/rehype-prism/blob/fb4174fce30a1cde8d784fa94e7c04d8a7fa6d28/index.js

// MIT License

// Copyright (c) 2017 Mapbox

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

'use strict';

const visit = require('unist-util-visit');
const nodeToString = require('hast-util-to-string');
const refractor = require('refractor');

module.exports = (options) => {
  options = options || {};

  if (options.alias) {
    refractor.alias(options.alias);
  }

  return (tree) => {
    visit(tree, 'element', (node, index, parent) => {
      if (typeof parent === 'undefined' ||
          parent.tagName !== 'pre' ||
          node.tagName !== 'code'
      ) {
        return;
      }
      const languagePrefix = 'language-';
      const langClass = ((node.properties.className || []).find(item => item.startsWith(languagePrefix)) || '').toLowerCase();
      if (langClass.length === 0) {
        return;
      }
      const lang = langClass.substr(languagePrefix.length);
      try {
        parent.properties.className = (parent.properties.className || []).concat(langClass);
        const result = refractor.highlight(nodeToString(node), lang);
        node.children = result;
      } catch (err) {
        if (options.ignoreMissing && /Unknown language/.test(err.message)) {
          return;
        }
        throw err;
      }
    });
  };
};
