/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';

export default class NodeIdentityTemplate extends Helper {
  compute([_name], { partition = 'default', canUsePartitions = false, canUseNspaces = false }) {
    const name = _name || '';
    if (canUsePartitions) {
      let block = `partition "${partition}" {\n`;
      if (canUseNspaces) {
        block += `  namespace "default" {\n    node "${name}" {\n      policy = "write"\n    }\n  }\n`;
        block += `  namespace_prefix "" {\n    service_prefix "" {\n      policy = "read"\n    }\n  }\n`;
      } else {
        block += `  node "${name}" {\n    policy = "write"\n  }\n`;
        block += `  service_prefix "" {\n    policy = "read"\n  }\n`;
      }
      block += `}`;
      return block;
    } else if (canUseNspaces) {
      return (
        `namespace "default" {\n  node "${name}" {\n    policy = "write"\n  }\n}\n` +
        `namespace_prefix "" {\n  service_prefix "" {\n    policy = "read"\n  }\n}`
      );
    } else {
      return (
        `node "${name}" {\n  policy = "write"\n}\n` + `service_prefix "" {\n  policy = "read"\n}`
      );
    }
  }
}
