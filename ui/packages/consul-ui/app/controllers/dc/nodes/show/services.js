import Controller from '@ember/controller';
import { get, computed } from '@ember/object';

export default class ServicesController extends Controller {
  queryParams = {
    search: {
      as: 'filter',
      replace: true,
    },
  };

  @computed('item.Checks.[]')
  get checks() {
    const checks = {};
    get(this, 'item.Checks')
      .filter(item => {
        return item.ServiceID !== '';
      })
      .forEach(item => {
        if (typeof checks[item.ServiceID] === 'undefined') {
          checks[item.ServiceID] = [];
        }
        checks[item.ServiceID].push(item);
      });

    return checks;
  }
}
