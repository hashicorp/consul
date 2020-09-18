import Component from '@ember/component';

export default Component.extend({
  tagName: '',
  actions: {
    createNewLabel: function(template, term) {
      return template.replace(/{{term}}/g, term);
    },
    isUnique: function(items, term) {
      return !items.findBy('Name', term);
    },
  },
});
