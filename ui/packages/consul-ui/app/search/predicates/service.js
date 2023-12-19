/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default {
  Name: (item) => item.Name,
  Tags: (item) => item.Tags || [],
  PeerName: (item) => item.PeerName,
  Partition: (item) => item.Partition,
};
