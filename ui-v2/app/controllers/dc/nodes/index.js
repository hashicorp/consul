import Controller from '@ember/controller';

export default Controller.extend({
  queryParams: {
    sortBy: 'sort',
    search: {
      as: 'filter',
      replace: true,
    },
  },
});
