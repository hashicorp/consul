import Component from '@ember/component';

export default Component.extend({
  tagName: '',

  shouldShowPermissionForm: false,

  actions: {
    createNewLabel: function(template, term) {
      return template.replace(/{{term}}/g, term);
    },
    isUnique: function(items, term) {
      return !items.findBy('Name', term);
    },
    add: function(name, changeset, value) {
      if (!(changeset.get(name) || []).includes(value) && value.isNew) {
        changeset.pushObject(name, value);
        changeset.validate();
      }
    },
    delete: function(name, changeset, value) {
      if ((changeset.get(name) || []).includes(value)) {
        changeset.removeObject(name, value);
        changeset.validate();
      }
    },
  },
});
