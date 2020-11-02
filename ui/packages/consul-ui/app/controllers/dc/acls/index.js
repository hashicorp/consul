import { action } from '@ember/object';
import Controller from '@ember/controller';
export default class IndexController extends Controller {
  queryParams = {
    filterBy: {
      as: 'type',
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };

  @action
  sendClone(item) {
    this.send('clone', item);
  }
}
