import Controller from '@ember/controller';
export default Controller.extend({
  actions: {
    close: function() {
      this.transitionToRoute('index');
    },
  },
});
