/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default class ToRouteHelper extends Helper {
  @service('router') router;
  @service('env') env;

  compute([url]) {
    const info = this.router.recognize(`${this.env.var('rootURL')}${url}`);
    return info.name;
  }
}
