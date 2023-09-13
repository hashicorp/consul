import Service, { inject as service } from '@ember/service';
import { getOwner } from '@ember/application';
import { match } from 'consul-ui/decorators/data-source';

export default class HttpService extends Service {
  @service('client/http') client;
  @service('data-source/protocols/http/blocking') type;

  source(src, configuration) {
    const route = match(src);
    let find;
    this.client.request((request) => {
      find = route.cb(route.params, getOwner(this), request);
    });
    return this.type.source(find, configuration);
  }
}
