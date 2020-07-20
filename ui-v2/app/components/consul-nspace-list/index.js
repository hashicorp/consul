import Component from '@ember/component';

export default Component.extend({
  tagName: '',
  actions: {
    isLinkable: function(item) {
      return !item.DeletedAt;
    },
  },
});
