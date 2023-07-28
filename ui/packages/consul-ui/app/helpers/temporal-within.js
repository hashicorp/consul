import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default class TemporalWithinHelper extends Helper {
  @service('temporal') temporal;
  compute(params, hash) {
    return this.temporal.within(params, hash);
  }
}
