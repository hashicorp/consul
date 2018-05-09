import Component from '@ember/component';
import { computed, set, get } from '@ember/object';

const size = 336;
const insetSize = size / 2 - 8;
const inset = function(num) {
  return insetSize * num;
};
const milliseconds = function(num, max) {
  return max > 0 ? parseInt(max * num) / 100 : 0;
};
export default Component.extend({
  size: size,
  tomography: 0,
  max: -999999999,
  init: function() {
    this._super(...arguments);
    this.circle = [inset(1), inset(0.25), inset(0.5), inset(0.75), inset(1)];
    this.labels = [inset(-0.25), inset(-0.5), inset(-0.75), inset(-1)];
  },
  milliseconds: computed('distances', 'max', function() {
    const max = get(this, 'max');
    return [
      milliseconds(25, max),
      milliseconds(50, max),
      milliseconds(75, max),
      milliseconds(100, max),
    ];
  }),
  distances: computed('tomography', function() {
    const tomography = this.get('tomography');
    let distances = tomography.distances || [];
    distances.forEach((d, i) => {
      if (d.distance > get(this, 'max')) {
        set(this, 'max', d.distance);
      }
    });
    if (tomography.n > 360) {
      let n = distances.length;
      // We have more nodes than we want to show, take a random sampling to keep
      // the number around 360.
      const sampling = 360 / tomography.n;
      distances = distances.filter(function(_, i) {
        return i == 0 || i == n - 1 || Math.random() < sampling;
      });
    }
    return distances.map((d, i) => {
      return {
        rotate: i * 360 / distances.length,
        y2: -insetSize * (d.distance / get(this, 'max')),
        node: d.node,
        distance: d.distance,
        segment: d.segment,
      };
    });
  }),
});
