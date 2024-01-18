/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';

export default class LinkToHcpBannerComponent extends Component {
  @action
  onDismiss() {
    console.log('Dismissed');
  }
  @action
  onClusterLink() {
    // TODO: CC-7147: Open simplified modal
  }
}
