/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class BreadcrumbsBackButtonComponent extends Component {
  @service router;

  @action
  goBack() {
    const { route, model } = this.args;
    if (model !== undefined) {
      this.router.transitionTo(route, model);
    } else {
      this.router.transitionTo(route);
    }
  }
}
