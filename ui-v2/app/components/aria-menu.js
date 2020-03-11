import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set } from '@ember/object';
import { next } from '@ember/runloop';

const TAB = 9;
const ENTER = 13;
const ESC = 27;
const SPACE = 32;
const END = 35;
const HOME = 36;
const ARROW_UP = 38;
const ARROW_DOWN = 40;

const keys = {
  vertical: {
    [ARROW_DOWN]: function($items, i = -1) {
      return (i + 1) % $items.length;
    },
    [ARROW_UP]: function($items, i = 0) {
      if (i === 0) {
        return $items.length - 1;
      } else {
        return i - 1;
      }
    },
    [HOME]: function($items, i) {
      return 0;
    },
    [END]: function($items, i) {
      return $items.length - 1;
    },
  },
  horizontal: {},
};
const COMPONENT_ID = 'component-aria-menu-';
// ^menuitem supports menuitemradio and menuitemcheckbox
const MENU_ITEMS = '[role^="menuitem"]';
export default Component.extend({
  tagName: '',
  dom: service('dom'),
  router: service('router'),
  guid: '',
  expanded: false,
  orientation: 'vertical',
  keyboardAccess: true,
  init: function() {
    this._super(...arguments);
    set(this, 'guid', this.dom.guid(this));
    this._listeners = this.dom.listeners();
    this._routelisteners = this.dom.listeners();
  },
  didInsertElement: function() {
    // TODO: How do you detect whether the children have changed?
    // For now we know that these elements exist and never change
    this.$menu = this.dom.element(`#${COMPONENT_ID}menu-${this.guid}`);
    const labelledBy = this.$menu.getAttribute('aria-labelledby');
    this.$trigger = this.dom.element(`#${labelledBy}`);
    this._routelisteners.add(this.router, {
      routeWillChange: () => this.actions.close.apply(this, [{}]),
    });
  },
  willDestroyElement: function() {
    this._super(...arguments);
    this._listeners.remove();
    this._routelisteners.remove();
  },
  actions: {
    keypressClick: function(e) {
      e.target.dispatchEvent(new MouseEvent('click'));
    },
    keypress: function(e) {
      // If the event is from the trigger and its not an opening/closing
      // key then don't do anything
      if (![ENTER, SPACE, ARROW_UP, ARROW_DOWN].includes(e.keyCode)) {
        return;
      }
      e.stopPropagation();
      // Also we may do this but not need it if we return early below
      // although once we add support for [A-Za-z] it unlikely we won't use
      // the keypress
      // TODO: We need to use > somehow here so we don't select submenus
      const $items = [...this.dom.elements(MENU_ITEMS, this.$menu)];
      if (!this.expanded) {
        this.$trigger.dispatchEvent(new MouseEvent('click'));
        if (e.keyCode === ENTER || e.keyCode === SPACE) {
          $items[0].focus();
          return;
        }
      }
      // this will prevent anything happening if you haven't pushed a
      // configurable key
      if (typeof keys[this.orientation][e.keyCode] === 'undefined') {
        return;
      }
      // prevent any scroll, or default actions
      e.preventDefault();
      const $focused = this.dom.element(`${MENU_ITEMS}:focus`, this.$menu);
      let i;
      if ($focused) {
        i = $items.findIndex(function($item) {
          return $item === $focused;
        });
      }
      const $next = $items[keys[this.orientation][e.keyCode]($items, i)];
      $next.focus();
    },
    // TODO: The argument here needs to change to an event
    // see toggle-button.change
    change: function(e) {
      const open = e.target.checked;
      if (open) {
        this.actions.open.apply(this, [e]);
      } else {
        this.actions.close.apply(this, [e]);
      }
    },
    close: function(e) {
      this._listeners.remove();
      set(this, 'expanded', false);
      // TODO: Find a better way to do this without using next
      // This is needed so when you press shift tab to leave the menu
      // and go to the previous button, it doesn't focus the trigger for
      // the menu itself
      next(() => {
        this.$trigger.removeAttribute('tabindex');
      });
    },
    open: function(e) {
      set(this, 'expanded', true);
      // Take the trigger out of the tabbing whilst the menu is open
      this.$trigger.setAttribute('tabindex', '-1');
      this._listeners.add(this.dom.document(), {
        keydown: e => {
          // Keep focus on the trigger when you close via ESC
          if (e.keyCode === ESC) {
            this.$trigger.focus();
          }
          if (e.keyCode === TAB || e.keyCode === ESC) {
            this.$trigger.dispatchEvent(new MouseEvent('click'));
            return;
          }
          if (!this.keyboardAccess) {
            return;
          }
          this.actions.keypress.apply(this, [e]);
        },
      });
    },
  },
});
