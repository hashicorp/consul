/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';

const size = 336;
const insetSize = size / 2 - 8;
const inset = function (num) {
  return insetSize * num;
};
const milliseconds = function (num, max) {
  return max > 0 ? parseInt(max * num) / 100 : 0;
};
export default class TomographyGraph extends Component {
  @tracked max = -999999999;
  size = size;

  circle = [inset(1), inset(0.25), inset(0.5), inset(0.75), inset(1)];
  labels = [inset(-0.25), inset(-0.5), inset(-0.75), inset(-1)];

  get milliseconds() {
    const distances = this.args.distances || [];
    const max = distances.reduce((prev, d) => Math.max(prev, d.distance), this.max);
    return [25, 50, 75, 100].map((item) => milliseconds(item, max));
  }

  get distances() {
    let distances = this.args.distances || [];
    const max = distances.reduce((prev, d) => Math.max(prev, d.distance), this.max);
    const len = distances.length;
    if (len > 360) {
      // We have more nodes than we want to show, take a random sampling to keep
      // the number around 360.
      const sampling = 360 / len;
      distances = distances.filter(function (_, i) {
        return i == 0 || i == len - 1 || Math.random() < sampling;
      });
    }
    return distances.map((d, i) => {
      return {
        rotate: (i * 360) / distances.length,
        y2: insetSize * (d.distance / max),
        node: d.node,
        distance: d.distance,
        segment: d.segment,
      };
    });
  }
}
