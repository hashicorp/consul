import Helper from '@ember/component/helper';

export default class DomPosition extends Helper {
  compute([target], { from, offset = false }) {
    return e => {
      if (typeof target === 'function') {
        let rect;
        let $el;
        if (offset) {
          $el = e.currentTarget;
          rect = {
            width: $el.offsetWidth,
            left: $el.offsetLeft,
            height: $el.offsetHeight,
            top: $el.offsetTop,
          };
        } else {
          $el = e.target;
          rect = $el.getBoundingClientRect();
          if (typeof from !== 'undefined') {
            const fromRect = from.getBoundingClientRect();
            rect.x = rect.x - fromRect.x;
            rect.y = rect.y - fromRect.y;
          }
        }
        return target(rect);
      }
    };
  }
}
