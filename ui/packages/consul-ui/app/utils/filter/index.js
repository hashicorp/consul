import setHelpers from 'mnemonist/set';

const createPossibles = function(predicates) {
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
const sanitize = function(values, possibles) {
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
const execute = function(item, values, predicates) {
  // every/and the top level values
  return Object.entries(values).every(([key, values]) => {
    let predicate = predicates[key];
    if (typeof predicate === 'function') {
      return predicate(item, values);
    } else {
      // if the top level values can have multiple values some/or them
      return values.some(val => predicate[val](item, val));
    }
  });
};
export const andOr = predicates => {
  const possibles = createPossibles(predicates);
  return () => values => {
    values = sanitize(values, possibles);
    return item => {
      return execute(item, values, predicates);
    };
  };
};
