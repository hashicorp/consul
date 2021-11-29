import Helper from 'ember-can/helpers/can';
import { get } from '@ember/object';

import { camelize } from '@ember/string';
export const is = (helper, [abilityString, model], properties) => {
  let { abilityName, propertyName } = helper.can.parse(abilityString);
  let ability = helper.can.abilityFor(abilityName, model, properties);

  if(typeof ability.getCharacteristicProperty === 'function') {
    propertyName = ability.getCharacteristicProperty(propertyName);
  } else {
    propertyName = camelize(`is-${propertyName}`);
  }

  helper._removeAbilityObserver();
  helper._addAbilityObserver(ability, propertyName);

  return get(ability, propertyName);
}
export default Helper.extend({
  compute([abilityString, model], properties) {
    return is(this, [abilityString, model], properties);
  },
});
