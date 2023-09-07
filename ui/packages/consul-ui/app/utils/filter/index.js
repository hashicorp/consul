/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import setHelpers from 'mnemonist/set';

const createPossibles = function (predicates) {
  // create arrays of allowed values
  return Object.entries(predicates).reduce((prev, [key, value]) => {
    if (typeof value !== 'function') {
      prev[key] = new Set(Object.keys(value));
    } else {
      prev[key] = null;
    }
    return prev;
  }, {});
};
const sanitize = function (values, possibles) {
  return Object.keys(possibles).reduce((prev, key) => {
    // only set the value if the value has a length of > 0
    const value = typeof values[key] === 'undefined' ? [] : values[key];
    if (value.length > 0) {
      if (possibles[key] !== null) {
        // only include possible values
        prev[key] = [...setHelpers.intersection(possibles[key], new Set(value))];
      } else {
        // only unique values
        prev[key] = [...new Set(value)];
      }
    }
    return prev;
  }, {});
};
const execute = function (item, values, predicates) {
  // every/and the top level values
  return Object.entries(values).every(([key, values]) => {
    let predicate = predicates[key];
    if (typeof predicate === 'function') {
      return predicate(item, values);
    } else {
      // if the top level values can have multiple values some/or them
      return values.some((val) => predicate[val](item, val));
    }
  });
};
// exports a function that requires a hash of predicates passed in
export const andOr = (predicates) => {
  // figure out all possible values from the hash of predicates
  const possibles = createPossibles(predicates);
  return (values) => {
    // this is what is called post injection
    // the actual user values are passed in here so 'sanitize' them which is
    // basically checking against the possibles
    values = sanitize(values, possibles);
    // this is your actual filter predicate
    return (item) => {
      return execute(item, values, predicates);
    };
  };
};
