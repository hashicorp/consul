/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

// simple mustache regexp `/{{item.Name}}/`
const templateRe = /{{([A-Za-z.0-9_-]+)}}/g;
let render;
export default class RenderTemplateHelper extends Helper {
  @service('encoder') encoder;
  constructor() {
    super(...arguments);
    if (typeof render !== 'function') {
      render = this.encoder.createRegExpEncoder(templateRe, encodeURIComponent, false);
    }
  }

  compute([template, vars]) {
    return render(template, vars);
  }
}
