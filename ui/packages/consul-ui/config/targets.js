'use strict';
// async/await support came with the below specified versions for Chrome,
// Firefox and Edge. Async/await is is the newest ES6 feature we are not
// transpiling. Safari's template literal support is a little problematic during
// v12 in that it has a GC bug for tagged template literals. We don't currently
// rely on this functionality so the bug wouldn't effect us, but in order to use
// browser versions as a measure for ES6 features we need to specify Safari 13
// for native, non-transpiled template literals. In reality template literals
// came in Safari 9.1. Safari's async/await support came in Safari 10, so thats
// the earliest Safari we cover in reality here.
module.exports = {
  browsers: ['Chrome 55', 'Firefox 53', 'Safari 13', 'Edge 15'],
};
