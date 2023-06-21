/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { helper } from '@ember/component/helper';
import { get } from '@ember/object';

/**
 * Datacenters can be an array of datacenters.
 * Anything that isn't an array means 'All', even an empty array.
 */
export function datacenters(params, hash = {}) {
  const datacenters = get(params[0], 'Datacenters');
  if (!Array.isArray(datacenters) || datacenters.length === 0) {
    return [hash['global'] || 'All'];
  }
  return get(params[0], 'Datacenters');
}

export default helper(datacenters);
