import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default Helper.extend({
  encoder: service('encoder'),
  compute(params, hash) {
    return this.encoder.uriJoin(params);
  },
});
