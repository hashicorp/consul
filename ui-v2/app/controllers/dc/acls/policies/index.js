import Controller from '@ember/controller';
export default Controller.extend({
  queryParams: {
    sortBy: 'sort',
    dc: 'dc',
    type: 'type',
    search: {
      as: 'filter',
      replace: true,
    },
  },
});
