import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { schedule } from '@ember/runloop';

const ENTER = 13;
const SELECTED = 'li.selected';
const ANIMATABLE = 'animatable';
export default Component.extend({
  name: 'tab',
  tagName: '',
  dom: service('dom'),
  init: function() {
    this._super(...arguments);
    this.guid = this.dom.guid(this);
  },
  didInsertElement: function() {
    this._super(...arguments);
    this.$nav = this.dom.element(`#${this.guid}`);
    this.select(this.dom.element(SELECTED, this.$nav));
    this.$nav.classList.add(ANIMATABLE);
  },
  didUpdateAttrs: function() {
    schedule('afterRender', () => this.select(this.dom.element(SELECTED, this.$nav)));
  },
  select: function($el) {
    this.dom.style(
      {
        '--selected-width': $el.offsetWidth,
        '--selected-left': $el.offsetLeft,
        '--selected-height': $el.offsetHeight,
        '--selected-top': $el.offsetTop,
      },
      this.$nav
    );
  },
  actions: {
    keydown: function(e) {
      if (e.keyCode === ENTER) {
        e.target.dispatchEvent(new MouseEvent('click'));
      }
    },
  },
});
