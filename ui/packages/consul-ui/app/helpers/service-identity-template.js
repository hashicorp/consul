/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';

export default class ServiceIdentityTemplate extends Helper {
  compute(
    [_name],
    { partition = 'default', nspace = 'default', canUsePartitions = false, canUseNspaces = false }
  ) {
    const name = _name || '';
    const indent = (text, level = 1) =>
      text
        .split('\n')
        .map((line) => '  '.repeat(level) + line)
        .join('\n');

    const baseBlock = () => {
      return [
        `service "${name}" {\n  policy = "write"\n}`,
        `service "${name}-sidecar-proxy" {\n  policy = "write"\n}`,
        `service_prefix "" {\n  policy = "read"\n}`,
        `node_prefix "" {\n  policy = "read"\n}`,
      ].join('\n');
    };

    if (canUsePartitions) {
      let block = `partition "${partition}" {\n`;

      if (canUseNspaces) {
        block += indent(`namespace "${nspace}" {\n${indent(baseBlock(), 1)}\n}`);
      } else {
        block += indent(baseBlock());
      }

      block += `\n}`;
      return block;
    } else if (canUseNspaces) {
      return `namespace "${nspace}" {\n${indent(baseBlock())}\n}`;
    } else {
      return baseBlock();
    }
  }
}
