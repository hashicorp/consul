import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import config from 'consul-ui/config/environment';

export default class ToRouteHelper extends Helper {
  @service('router') router;

  compute([url]) {
    const info = this.router.recognize(`${config.rootURL}${url}`);
    return info.name;
  }
}
