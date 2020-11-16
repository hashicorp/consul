import Controller from '@ember/controller';

export default class IndexController extends Controller {
  queryParams = {
    sortBy: 'sort',
    dc: 'dc',
    kind: 'kind',
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
