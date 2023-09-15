/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';
const STYLE_RULE = 1;
const getCustomProperties = function () {
  return Object.fromEntries(
    [...document.styleSheets].reduce(
      (prev, item) =>
        prev.concat(
          [...item.cssRules]
            .filter((item) => item.type === STYLE_RULE)
            .reduce((prev, rule) => {
              const props = [...rule.style]
                .filter((prop) => prop.startsWith('--'))
                .map((prop) => [prop.trim(), rule.style.getPropertyValue(prop).trim()]);
              return [...prev, ...props];
            }, [])
        ),
      []
    )
  );
};
const props = getCustomProperties();
export default modifier(function ($element, [returns], hash) {
  const re = new RegExp(`^--${hash.prefix || '.'}${hash.group || ''}+`);
  const obj = {};
  Object.entries(props).forEach(([key, value]) => {
    const res = key.match(re);
    if (res) {
      let prop = res[0];
      if (prop.charAt(prop.length - 1) === '-') {
        prop = prop.substr(0, prop.length - 1);
      }
      if (hash.group) {
        if (typeof obj[prop] === 'undefined') {
          obj[prop] = {};
        }
        obj[prop][key] = value;
      } else {
        obj[key] = value;
      }
    }
  });
  returns(obj);
});
