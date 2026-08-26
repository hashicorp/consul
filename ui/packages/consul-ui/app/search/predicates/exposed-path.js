/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  Protocol: (item) => item.Protocol,
  Path: (item) => item.Path,
  ListenerPort: (item) => (item.ListenerPort || '').toString(),
  LocalPathPort: (item) => (item.LocalPathPort || '').toString(),
};
