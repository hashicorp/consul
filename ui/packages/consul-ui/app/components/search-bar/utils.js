/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export const diff = (a, b) => {
  return a.filter((item) => !b.includes(item));
};
/**
 * filters accepts the args.filter @attribute which is shaped like
 * {filterName: {default: ['Node', 'Address'], value: ['Address']}, ...}
 * It will turn this into an array of 'filters' shaped like
 * [{key: 'filterName', value: 'Address', selected: ["Node"]}]
 * importantly 'selected' isn't what is currently 'selected' it is what selected
 * will be once you remove this filter
 * There is more explanation in the unit tests for this function so thats worthwhile
 * checking if you are in amongst this
 */
export const filters = (filters) => {
  return Object.entries(filters)
    .filter(([key, value]) => {
      if (key === 'searchproperty') {
        return diff(value.default, value.value).length > 0;
      }
      return (value.value || []).length > 0;
    })
    .reduce((prev, [key, value]) => {
      return prev.concat(
        value.value.map((item) => {
          const obj = {
            key: key,
            value: item,
          };
          if (key !== 'searchproperty') {
            obj.selected = diff(value.value, [item]);
          } else {
            obj.selected = value.value.length === 1 ? value.default : diff(value.value, [item]);
          }
          return obj;
        })
      );
    }, []);
};
