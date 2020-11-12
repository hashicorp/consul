import Controller from '@ember/controller';

export default class DcServicesInstanceUpstreamsController extends Controller {
  queryParams = {
    sortBy: 'sort',
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
