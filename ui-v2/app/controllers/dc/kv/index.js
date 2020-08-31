import Controller from '@ember/controller';
export default Controller.extend({
  queryParams: {
    search: {
      as: 'filter',
      replace: true,
    },
  },
});
