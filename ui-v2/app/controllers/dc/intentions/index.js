import Controller from '@ember/controller';

export default Controller.extend({
  queryParams: {
    sortBy: 'sort',
    access: 'access',
    search: {
      as: 'filter',
      replace: true,
    },
  },
});
