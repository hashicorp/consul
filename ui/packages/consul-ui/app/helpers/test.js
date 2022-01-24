import Helper from 'ember-can/helpers/can';
import { is } from 'consul-ui/helpers/is';

export default Helper.extend({
  compute([abilityString, model], properties) {
    switch(true) {
      case abilityString.startsWith('can '):
        return super.compute([abilityString.substr(4), model], properties);
      case abilityString.startsWith('is '):
        return is(this, [abilityString.substr(3), model], properties);
    }
    throw new Error(`${abilityString} is not supported by the 'test' helper.`);
  },
});
