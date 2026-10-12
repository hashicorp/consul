/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';

// Order matches the Hds::Tabs::Tab order rendered in the template below.
const TAB_EVENTS = ['GENERATE', 'INITIATE'];

export default class ConsulPeerFormComponent extends Component {
  @action
  onClickTab(dispatch, event, tabIndex) {
    dispatch(TAB_EVENTS[tabIndex]);
  }
}
