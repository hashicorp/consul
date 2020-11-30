import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';
export default Helper.extend({
  dom: service('dom'),
  compute: function([target, from], hash) {
    const $target = this.dom.element(target);
    const $from = this.dom.element(from);
    const fromRect = $from.getBoundingClientRect();
    const rect = $target.getBoundingClientRect();
    rect.x = rect.x - fromRect.x;
    rect.y = rect.y - fromRect.y;
    return rect;
  },
});
