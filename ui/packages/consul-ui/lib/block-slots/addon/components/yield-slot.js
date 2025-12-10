import { computed, get } from '@ember/object';
import { reads } from '@ember/object/computed';
import Component from '@ember/component';
import layout from '../templates/components/yield-slot';
import Slots from '../mixins/slots';
const YieldSlotComponent = Component.extend({
  layout,
  tagName: '',
  _name: reads('name'),
  _blockParams: reads('params'),
  _parentView: computed(function () {
    return this.nearestOfType(Slots);
  }),
  isActive: computed('_parentView._slots.[]', '_name', function () {
    return get(this, '_parentView._slots').includes(get(this, '_name'));
  }),
});

export default YieldSlotComponent;
