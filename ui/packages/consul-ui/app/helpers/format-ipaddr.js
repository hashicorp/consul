/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
import { processIpAddress } from 'consul-ui/utils/process-ip-address';

export default helper(function formatIpaddr([ipaddress]) {
  const value = processIpAddress(ipaddress);
  return value ? value : '';
});
