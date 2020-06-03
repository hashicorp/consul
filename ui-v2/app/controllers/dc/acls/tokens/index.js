import Controller from '@ember/controller';
export default Controller.extend({
  queryParams: {
    search: {
      as: 'filter',
      replace: true,
    },
  },
  actions: {
    sendClone: function(item) {
      this.send('clone', item);
    },
  },
});
