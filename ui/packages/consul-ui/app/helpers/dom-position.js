import Helper from '@ember/component/helper';

export default class DomPosition extends Helper {
  compute([target], { from }) {
    if (typeof target === 'function') {
      return entry => {
        const $target = entry.target;
        let rect = $target.getBoundingClientRect();
        if (typeof from !== 'undefined') {
          const fromRect = from.getBoundingClientRect();
          rect.x = rect.x - fromRect.x;
          rect.y = rect.y - fromRect.y;
        }
        return target(rect);
      };
    }
  }
}
