import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default class UriHelper extends Helper {
  @service('encoder') encoder;

  compute(params, hash) {
    return this.encoder.uriJoin(params);
  }
}
