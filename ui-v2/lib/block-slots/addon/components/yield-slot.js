import { computed, get } from '@ember/object';
import Component from '@ember/component';
import layout from '../templates/components/yield-slot';
import Slots from '../mixins/slots';
const YieldSlotComponent = Component.extend({
  layout,
  tagName: '',
  _name: computed('__name', 'name', function() {
    return this.name || this.__name;
  }),
  _blockParams: computed('__blockParams', 'params', function() {
    return this.params || this.__blockParams;
  }),
  _parentView: computed(function() {
    return this.nearestOfType(Slots);
  }),
  isActive: computed('_parentView._slots.[]', '_name', function() {
    return get(this, '_parentView._slots').includes(get(this, '_name'));
  }),
});

YieldSlotComponent.reopenClass({
  positionalParams: ['__name', '__blockParams'],
});

export default YieldSlotComponent;
