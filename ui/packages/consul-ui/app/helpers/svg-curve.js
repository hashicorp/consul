/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
// arguments should be a list of {x: numLike, y: numLike} points
// numLike meaning they should be numbers (or numberlike strings i.e. "1" vs 1)
const curve = function () {
  const args = [...arguments];
  // our arguments are destination first control points last
  // SVGs are control points first destination last
  // we 'shift,push' to turn that around and then map
  // through the values to convert it to 'x y, x y' etc
  // whether the curve is cubic-bezier (C) or quadratic-bezier (Q)
  // then depends on the amount of control points
  // `Q|C x y, x y, x y` etc
  return `${arguments.length > 2 ? `C` : `Q`} ${args
    .concat(args.shift())
    .map((p) => Object.values(p).join(' '))
    .join(',')}`;
};
const move = function (d) {
  return `
    M ${d.x} ${d.y}
  `;
};

export default helper(function ([dest], hash) {
  const src = hash.src || { x: 0, y: 0 };
  const type = hash.type || 'cubic';
  let args = [
    dest,
    {
      x: (src.x + dest.x) / 2,
      y: src.y,
    },
  ];
  if (type === 'cubic') {
    args.push({
      x: args[1].x,
      y: dest.y,
    });
  }
  return `${move(src)}${curve(...args)}`;
});
