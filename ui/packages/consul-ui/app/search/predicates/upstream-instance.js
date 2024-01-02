/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  DestinationName: (item, value) => item.DestinationName,
  LocalBindAddress: (item, value) => item.LocalBindAddress,
  LocalBindPort: (item, value) => item.LocalBindPort.toString(),
};
