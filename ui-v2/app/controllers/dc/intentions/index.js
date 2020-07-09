import Controller from '@ember/controller';
export default Controller.extend({
  queryParams: {
    filterBy: {
      as: 'action',
    },
    search: {
      as: 'filter',
      replace: true,
    },
  },
});
