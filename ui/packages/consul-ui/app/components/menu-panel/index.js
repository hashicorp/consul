import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { next } from '@ember/runloop';
import { set } from '@ember/object';

import Slotted from 'block-slots';

export default Component.extend(Slotted, {
  tagName: '',
  dom: service('dom'),
  isConfirmation: false,
  actions: {
    connect: function($el) {
      next(() => {
        if(!this.isDestroyed) {
          // if theres only a single choice in the menu and it doesn't have an
          // immediate button/link/label to click then it will be a
          // confirmation/informed action
          const isConfirmationMenu = this.dom.element(
            'li:only-child > [role="menu"]:first-child',
            $el
          );
          set(this, 'isConfirmation', typeof isConfirmationMenu !== 'undefined');
        }
      });
    },
    change: function(e) {
      const id = e.target.getAttribute('id');
      const $trigger = this.dom.element(`[for='${id}']`);
      const $panel = this.dom.element('[role=menu]', $trigger.parentElement);
      const $menuPanel = this.dom.closest('.menu-panel', $panel);
      if (e.target.checked) {
        $panel.style.display = 'block';
        const height = $panel.offsetHeight + 2;
        $menuPanel.style.maxHeight = $menuPanel.style.minHeight = `${height}px`;
      } else {
        $panel.style.display = null;
        $menuPanel.style.maxHeight = null;
        $menuPanel.style.minHeight = '0';
      }
    },
  },
});
