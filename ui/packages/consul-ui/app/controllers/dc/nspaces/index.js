import Controller from '@ember/controller';

export default class IndexController extends Controller {
  queryParams = {
    sortBy: 'sort',
    search: {
      as: 'filter',
    },
  };
}
