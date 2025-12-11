/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */
import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class DebugLayoutComponent extends Component {
  @service flashMessages;
}
