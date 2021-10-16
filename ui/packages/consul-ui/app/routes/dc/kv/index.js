import Route from 'consul-ui/routing/route';
import { action } from '@ember/object';
import isFolder from 'consul-ui/utils/isFolder';

export default class IndexRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    kind: 'kind',
    search: {
      as: 'filter',
      replace: true,
    },
  };

  beforeModel() {
    // we are index or folder, so if the key doesn't have a trailing slash
    // add one to force a fake findBySlug
    const params = this.paramsFor(this.routeName);
    const key = params.key || '/';
    if (!isFolder(key)) {
      return this.replaceWith(this.routeName, key + '/');
    }
  }

  @action
  error(e) {
    if (e.errors && e.errors[0] && e.errors[0].status == '404') {
      return this.transitionTo('dc.kv.index');
    }
    // let the route above handle the error
    return true;
  }
}
