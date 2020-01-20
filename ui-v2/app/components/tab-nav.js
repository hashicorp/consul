import Component from '@ember/component';

const ENTER = 13;
export default Component.extend({
  name: 'tab',
  tagName: '',
  actions: {
    keydown: function(e) {
      if (e.keyCode === ENTER) {
        e.target.dispatchEvent(new MouseEvent('click'));
      }
    },
  },
});
