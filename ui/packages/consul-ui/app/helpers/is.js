/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from 'ember-can/helpers/can';
import { get } from '@ember/object';

import { camelize } from '@ember/string';
export const is = (helper, [abilityString, model], properties) => {
  let { abilityName, propertyName } = helper.abilities.parse(abilityString);
  let ability = helper.abilities.abilityFor(abilityName, model, properties);

  if (typeof ability.getCharacteristicProperty === 'function') {
    propertyName = ability.getCharacteristicProperty(propertyName);
  } else {
    propertyName = camelize(`is-${propertyName}`);
  }

  return get(ability, propertyName);
};
export default class extends Helper {
  compute([abilityString, model], properties) {
    return is(this, [abilityString, model], properties);
  }
}
