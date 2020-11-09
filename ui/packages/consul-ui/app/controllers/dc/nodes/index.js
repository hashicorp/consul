import Controller from '@ember/controller';

export default class IndexController extends Controller {
  queryParams = {
    sortBy: 'sort',
    status: 'status',
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
