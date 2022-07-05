import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default class extends Helper {
  @service features;

  compute([feature]) {
    return this.features.isEnabled(feature);
  }
}
