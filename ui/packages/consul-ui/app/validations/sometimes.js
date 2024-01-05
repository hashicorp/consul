/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable no-prototype-builtins */
import { get } from '@ember/object';

export default function validateSometimes(validator, condition) {
  return guardValidatorWithCondition(validator);

  function guardValidatorWithCondition(validator) {
    return function (key, newValue, oldValue, changes, content) {
      let thisValue = {
        get(property) {
          if (property.includes('.')) {
            let changesValue = get(changes, property);
            if (typeof changesValue !== 'undefined') {
              return changesValue;
            }

            // Check if the `changes` value is explicitly undefined,
            // or if it's not present at all.
            let pathSegments = property.split('.');
            let propName = pathSegments.pop();
            let objPath = pathSegments.join('.');

            let obj = get(changes, objPath);
            if (obj && obj.hasOwnProperty && obj.hasOwnProperty(propName)) {
              return changesValue;
            }

            return get(content, property);
          }

          if (changes.hasOwnProperty(property)) {
            return get(changes, property);
          } else {
            return get(content, property);
          }
        },
      };

      if (condition.call(thisValue, changes, content)) {
        return validator(key, newValue, oldValue, changes, content);
      }
      return true;
    };
  }
}
