/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// values come from filter/predicates/kv, which matches on folder|key
const KIND_OPTIONS = [
  { value: 'folder', label: 'Folder' },
  { value: 'key', label: 'Key' },
];

export default class ConsulKvToolbar extends Component {
  get filterGroups() {
    return [{ key: 'kind', text: 'Type', options: KIND_OPTIONS }];
  }
}
