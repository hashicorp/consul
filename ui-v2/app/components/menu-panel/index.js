import Component from '@ember/component';
import { inject as service } from '@ember/service';

import Slotted from 'block-slots';

export default Component.extend(Slotted, {
  tagName: '',
  dom: service('dom'),
  actions: {
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
