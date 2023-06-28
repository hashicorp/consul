/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default {
  Node: (item) => item.Node,
  Address: (item) => item.Address,
  PeerName: (item) => item.PeerName,
  Meta: (item) => Object.entries(item.Meta || {}).reduce((prev, entry) => prev.concat(entry), []),
};
