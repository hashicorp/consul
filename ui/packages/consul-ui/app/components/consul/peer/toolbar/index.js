/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { schema } from 'consul-ui/models/peer';

// Labels match the Status column's badge, which renders the same states as
// `{{capitalize (lowercase State)}}` (see app/components/peerings/badge).
const STATE_OPTIONS = schema.State.allowedValues.map((state) => ({
  value: state.toLowerCase(),
  label: state.charAt(0) + state.slice(1).toLowerCase(),
}));

export default class ConsulPeerToolbar extends Component {
  get filterGroups() {
    return [{ key: 'state', text: 'Status', options: STATE_OPTIONS }];
  }
}
