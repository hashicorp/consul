import Controller from '@ember/controller';

export default class UpstreamsController extends Controller {
  queryParams = {
    sortBy: 'sort',
    instance: 'instance',
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
