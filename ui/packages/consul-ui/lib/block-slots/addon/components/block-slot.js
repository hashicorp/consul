import { readOnly } from '@ember/object/computed';
import Component from '@ember/component';
import { isPresent } from '@ember/utils';
import { get, set, defineProperty, computed } from '@ember/object';
import layout from '../templates/components/block-slot';
import Slots from '../mixins/slots';
import YieldSlot from './yield-slot';
const BlockSlot = Component.extend({
  layout,
  tagName: '',
  _name: computed('name', function() {
    return this.name;
  }),
  didInsertElement: function() {
    const slottedComponent = this.nearestOfType(Slots);
    if (!slottedComponent._isRegistered(this._name)) {
      slottedComponent._activateSlot(this._name);
      set(this, 'slottedComponent', slottedComponent);
      return;
    }
    const yieldSlot = this.nearestOfType(YieldSlot);
    if (yieldSlot) {
      set(this, '_yieldSlot', yieldSlot);
      // The slotted component will yield multiple times - once to register
      // the activate slots and once more per active slot - only display this
      // block when its associated slot is the one yielding
      const isTargetSlotYielding = get(yieldSlot, '_name') === this._name;
      set(this, 'isTargetSlotYielding', isTargetSlotYielding);

      // If the associated slot has block params, create a computed property
      // for each block param.  Technically this could be an unlimited, but
      // hbs lacks a spread operator so params are currently limited to 9
      // (see the yield in the block-slot template)
      //
      // Spread PR: https://github.com/wycats/handlebars.js/pull/1149
      const params = get(yieldSlot, '_blockParams');
      if (isTargetSlotYielding && isPresent(params)) {
        // p0 p1 p2...
        params.forEach((param, index) => {
          defineProperty(this, `p${index}`, readOnly(`_yieldSlot._blockParams.${index}`));
        });
      }
    }
  },
  willDestroyElement: function() {
    if (this.slottedComponent) {
      // Deactivate the yield slot using the slots interface when the block
      // is destroyed to allow the yield slot default {{else}} to take effect
      // dynamically
      this.slottedComponent._deactivateSlot(this._name);
    }
  },
});

export default BlockSlot;
