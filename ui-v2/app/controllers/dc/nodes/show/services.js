import Controller from '@ember/controller';
import { alias } from '@ember/object/computed';

export default Controller.extend({
  items: alias('item.Services'),
  queryParams: {
    search: {
      as: 'filter',
      replace: true,
    },
  },
});
