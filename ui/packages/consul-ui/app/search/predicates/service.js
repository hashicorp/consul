/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  Name: (item) => item.Name,
  Tags: (item) => item.Tags || [],
  PeerName: (item) => item.PeerName,
  Partition: (item) => item.Partition,
};
