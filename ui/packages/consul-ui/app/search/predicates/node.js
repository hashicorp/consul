/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  Node: (item) => item.Node,
  Address: (item) => item.Address,
  PeerName: (item) => item.PeerName,
  Meta: (item) => Object.entries(item.Meta || {}).reduce((prev, entry) => prev.concat(entry), []),
};
