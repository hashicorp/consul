/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class AppComponent extends Component {
  @service('dom') dom;

  constructor(args, owner) {
    super(...arguments);
    this.guid = this.dom.guid(this);
  }

  @action
  keypressClick(e) {
    e.target.dispatchEvent(new MouseEvent('click'));
  }

  @action
  focus(e) {
    const href = e.target.getAttribute('href');
    if (href.startsWith('#')) {
      e.preventDefault();
      this.dom.focus(href);
    }
  }

  @action
  unfocus(e) {
    e.target.blur();
  }
}
