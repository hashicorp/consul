import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default Helper.extend({
  dom: service('dom'),
  compute: function([selector], hash) {
    const $el = this.dom.element(selector);
    let $refs = [$el.offsetParent, $el];
    // TODO: helper probably need to accept a `reference=` option
    // with a selector to use as reference/root
    if (selector.startsWith('#resolver:')) {
      $refs.unshift($refs[0].offsetParent);
    }
    const pos = $refs.reduce(
      function(prev, item) {
        prev.x += item.offsetLeft;
        prev.y += item.offsetTop;
        return prev;
      },
      {
        x: 0,
        y: 0,
        height: $el.offsetHeight,
        width: $el.offsetWidth,
      }
    );
    return pos;
  },
});
