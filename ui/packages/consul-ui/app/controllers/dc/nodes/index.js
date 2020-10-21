import Controller from '@ember/controller';

export default Controller.extend({
  queryParams: {
    sortBy: 'sort',
    status: 'status',
    search: {
      as: 'filter',
      replace: true,
    },
  },
});
