import Component from '@ember/component';

const ENTER = 13;
export default Component.extend({
  tagName: '',
  keyboardAccess: false,
  actions: {
    keydown: function(e) {
      if (e.keyCode === ENTER) {
        e.target.dispatchEvent(new MouseEvent('click'));
      }
    },
  },
});
