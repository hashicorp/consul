import Component from '@ember/component';
import { setProperties, set } from '@ember/object';
import { inject as service } from '@ember/service';
import { schedule } from '@ember/runloop';

const ENTER = 13;
const SELECTED = 'li.selected';
export default Component.extend({
  name: 'tab',
  tagName: '',
  dom: service('dom'),
  isAnimatable: false,
  init: function() {
    this._super(...arguments);
    this.guid = this.dom.guid(this);
  },
  didInsertElement: function() {
    this._super(...arguments);
    this.$nav = this.dom.element(`#${this.guid}`);
    this.select(this.dom.element(SELECTED, this.$nav));
    set(this, 'isAnimatable', true);
  },
  didUpdateAttrs: function() {
    schedule('afterRender', () => this.select(this.dom.element(SELECTED, this.$nav)));
  },
  select: function($el) {
    if (!$el) {
      return;
    }
    setProperties(this, {
      selectedWidth: $el.offsetWidth,
      selectedLeft: $el.offsetLeft,
      selectedHeight: $el.offsetHeight,
      selectedTop: $el.offsetTop,
    });
  },
  actions: {
    keydown: function(e) {
      if (e.keyCode === ENTER) {
        e.target.dispatchEvent(new MouseEvent('click'));
      }
    },
  },
});
