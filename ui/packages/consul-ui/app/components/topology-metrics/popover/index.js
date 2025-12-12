/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';

export default class TopologyMetricsButton extends Component {
  // This component has no functionality yet, but we need it to
  // exist to attach the popover to it in the parent component.
  @tracked popover = null;
}
