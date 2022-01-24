import Service, { inject as service } from '@ember/service';
import { getOwner } from '@ember/application';
import { match } from 'consul-ui/decorators/data-source';

export default class HttpService extends Service {
  @service('data-source/protocols/http/blocking') type;

  source(src, configuration) {
    const route = match(src);
    const find = route.cb(route.params, getOwner(this));
    return this.type.source(find, configuration);
  }
}
