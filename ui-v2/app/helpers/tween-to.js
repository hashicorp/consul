import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default Helper.extend({
  ticker: service('ticker'),
  compute: function([props, id], hash) {
    return this.ticker.tweenTo(props, id);
  },
});
