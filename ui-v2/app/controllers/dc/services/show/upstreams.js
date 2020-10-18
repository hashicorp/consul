import Controller from '@ember/controller';

export default Controller.extend({
  queryParams: {
    sortBy: 'sort',
    instance: 'instance',
    search: {
      as: 'filter',
      replace: true,
    },
  },
});
