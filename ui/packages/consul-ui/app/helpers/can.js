import Helper from 'ember-can/helpers/can';

export default class extends Helper {
  _addAbilityObserver(ability, propertyName) {
    if(!this.isDestroyed && !this.isDestroying) {
      super._addAbilityObserver(...arguments);
    }
  }
}
