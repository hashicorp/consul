import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default Helper.extend({
  state: service('state'),
  compute([state, values], hash) {
    return this.state.matches(state, values);
  },
});
