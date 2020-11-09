import Controller from '@ember/controller';
export default class IndexController extends Controller {
  queryParams = {
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
