import Component from '@ember/component';
import { computed } from '@ember/object';

var size = 336;
var insetSize = size / 2 - 8;
var inset = function(num)
{
  return insetSize * num;
}
var max = -999999999;
var milliseconds = function(num)
{
  return (max > 0 ? (parseInt(max * 25) / 100) : 0)
};
export default Component.extend({
  size: 336,
  tomography: 0,
  circle: [
    inset(1),
    inset(.25),
    inset(.5),
    inset(.75),
    inset(1)
  ],
  labels: [
    inset(-.25),
    inset(-.5),
    inset(-.75),
    inset(-1)
  ],
  distances: computed(
    'tomography',
    function() {
      const tomography = this.get('tomography');
      tomography.distances.forEach(function (d, i) {
        if (d.distance > max) {
          max = d.distance;
        }
      });
      let distances = tomography.distances;
      let n = distances.length;
      if (tomography.n > 360) {
        // We have more nodes than we want to show, take a random sampling to keep
        // the number around 360.
        const sampling = 360 / tomography.n;
        distances = distances.filter(function (_, i) {
          return i == 0 || i == n - 1 || Math.random() < sampling
        });
        // Re-set n to the filtered size
        n = distances.length;
      }
      return distances.map(function (d, i) {
        return {
          rotate: (i * 360 / n),
          y2: (-insetSize * (d.distance / max)),
          node: d.node,
          distance: d.distance,
          segment: d.segment
        }
      });
    }
  )
});
