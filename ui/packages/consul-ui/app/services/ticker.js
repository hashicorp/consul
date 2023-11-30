/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import { Tween } from 'consul-ui/utils/ticker';

let map;
export default class TickerService extends Service {
  init() {
    super.init(...arguments);
    this.reset();
  }

  tweenTo(props, obj = '', frames, method) {
    // TODO: Right now we only support string id's
    // but potentially look at allowing passing of other objects
    // especially DOM elements
    const id = obj;
    if (!map.has(id)) {
      map.set(id, props);
      return props;
    } else {
      obj = map.get(id);
      if (obj instanceof Tween) {
        obj = obj.stop().getTarget();
      }
      map.set(id, Tween.to(obj, props, frames, method));
      return obj;
    }
  }

  // TODO: We'll try and use obj later for ticker bookkeeping
  destroy(obj) {
    this.reset();
    return Tween.destroy();
  }

  reset() {
    map = new Map();
  }
}
