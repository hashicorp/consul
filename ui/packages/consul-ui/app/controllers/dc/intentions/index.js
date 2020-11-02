import Controller from '@ember/controller';

export default class IndexController extends Controller {
  queryParams = {
    sortBy: 'sort',
    access: 'access',
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
