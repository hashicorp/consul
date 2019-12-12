import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set } from '@ember/object';
import { next } from '@ember/runloop';

const ARROW_DOWN = 40;
const ARROW_UP = 38;
const TAB = 9;
const ESC = 27;
const HOME = 36;
const END = 35;

const keys = {
  vertical: {
    [ARROW_DOWN]: function(i, $items) {
      return (i + 1) % $items.length;
    },
    [ARROW_UP]: function(i, $items) {
      if (i === 0) {
        return $items.length - 1;
      } else {
        return i - 1;
      }
    },
    [HOME]: function(i, $items) {
      return 0;
    },
    [END]: function(i, $items) {
      return $items.length - 1;
    },
  },
  horizontal: {},
};
export default Component.extend({
  tagName: '',
  dom: service('dom'),
  guid: '',
  expanded: false,
  direction: 'vertical',
  init: function() {
    this._super(...arguments);
    set(this, 'guid', this.dom.guid(this));
    this._listeners = this.dom.listeners();
  },
  didInsertElement: function() {
    // TODO: How do you detect whether the childnre have changed?
    // For now we know that these elements exist and never change
    const $ref = this.dom.element(`#aria-menu-${this.guid}`);
    this.$element = $ref.parentNode;
    this.$menu = this.dom.element('[role="menu"]', this.$element);
    const labelledBy = this.$menu.getAttribute('aria-labelledby');
    this.$trigger = this.dom.element(`#${labelledBy}`);
    $ref.remove();
  },
  willDestroyElement: function() {
    this._super(...arguments);
    this._listeners.remove();
  },
  actions: {
    // TODO: The argument here needs to change to an event
    // see toggle-button.change
    change: function(open) {
      if (open) {
        this.actions.open.apply(this, []);
      } else {
        this.actions.close.apply(this, []);
      }
    },
    close: function(e) {
      this._listeners.remove();
      set(this, 'expanded', false);
      // TODO: Find a better way to do this without using next
      next(() => {
        this.$trigger.removeAttribute('tabindex');
      });
    },
    open: function(e) {
      set(this, 'expanded', true);
      this.$trigger.setAttribute('tabindex', '-1');
      const direction = this;
      this._listeners.add(this.dom.document(), {
        keydown: e => {
          if (e.keyCode === ESC) {
            this.$trigger.focus();
          }
          if (e.keyCode === TAB || e.keyCode === ESC) {
            this.$trigger.dispatchEvent(new MouseEvent('click'));
            return;
          }
          if (typeof keys[this.direction][e.keyCode] === 'undefined') {
            return;
          }
          // TODO: We need to use > somehow here so we don't select submenus
          const $items = [...this.dom.elements('[role="menuitem"]', this.$menu)];
          const $focused = this.dom.element('[role="menuitem"]:focus', this.$menu);
          let $next = $items[0];
          if ($focused) {
            const i = $items.findIndex(function($item) {
              return $item === $focused;
            });
            $next = $items[keys[this.direction][e.keyCode](i, $items)];
          }
          $next.focus();
        },
      });
    },
  },
});
