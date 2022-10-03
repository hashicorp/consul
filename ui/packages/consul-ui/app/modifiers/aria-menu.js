import Modifier from 'ember-modifier';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

const TAB = 9;
const ESC = 27;
const END = 35;
const HOME = 36;
const ARROW_UP = 38;
const ARROW_DOWN = 40;

const keys = {
  vertical: {
    [ARROW_DOWN]: ($items, i = -1) => {
      return (i + 1) % $items.length;
    },
    [ARROW_UP]: ($items, i = 0) => {
      if (i === 0) {
        return $items.length - 1;
      } else {
        return i - 1;
      }
    },
    [HOME]: ($items, i) => {
      return 0;
    },
    [END]: ($items, i) => {
      return $items.length - 1;
    },
  },
  horizontal: {},
};

const MENU_ITEMS = '[role^="menuitem"]';

export default class AriaMenuModifier extends Modifier {
  @service('-document') doc;
  orientation = 'vertical';

  @action
  async keydown(e) {
    if (e.keyCode === ESC) {
      this.options.onclose(e);
      this.$trigger.focus();
      return;
    }
    const $items = [...this.element.querySelectorAll(MENU_ITEMS)];
    const pos = $items.findIndex(($item) => $item === this.doc.activeElement);
    if (e.keyCode === TAB) {
      if (e.shiftKey) {
        if (pos === 0) {
          this.options.onclose(e);
          this.$trigger.focus();
        }
      } else {
        if (pos === $items.length - 1) {
          await new Promise((resolve) => setTimeout(resolve, 0));
          this.options.onclose(e);
        }
      }
      return;
    }
    if (typeof keys[this.orientation][e.keyCode] === 'undefined') {
      return;
    }
    $items[keys[this.orientation][e.keyCode]($items, pos)].focus();
    e.stopPropagation();
    e.preventDefault();
  }

  @action
  async focus(e) {
    if (e.pointerType === '') {
      await Promise.resolve();
      this.keydown({
        keyCode: HOME,
        stopPropagation: () => {},
        preventDefault: () => {},
      });
    }
  }

  connect(params, named) {
    this.$trigger = this.doc.getElementById(this.element.getAttribute('aria-labelledby'));
    if (typeof named.openEvent !== 'undefined') {
      this.focus(named.openEvent);
    }
    this.doc.addEventListener('keydown', this.keydown);
  }

  disconnect() {
    this.doc.removeEventListener('keydown', this.keydown);
  }

  didReceiveArguments() {
    this.params = this.args.positional;
    this.options = this.args.named;
  }

  didInstall() {
    this.connect(this.args.positional, this.args.named);
  }

  willRemove() {
    this.disconnect();
  }
}
