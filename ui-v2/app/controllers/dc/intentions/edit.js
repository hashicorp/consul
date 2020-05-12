import Controller from '@ember/controller';
export default Controller.extend({
  actions: {
    route: function() {
      this.send(...arguments);
    },
  },
});
