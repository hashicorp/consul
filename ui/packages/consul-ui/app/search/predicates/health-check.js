/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

const asArray = function (arr) {
  return Array.isArray(arr) ? arr : arr.toArray();
};
export default {
  Name: (item) => item.Name,
  Node: (item) => item.Node,
  Service: (item) => item.ServiceName,
  CheckID: (item) => item.CheckID || '',
  ID: (item) => item.Service.ID || '',
  Notes: (item) => item.Notes,
  Output: (item) => item.Output,
  ServiceTags: (item) => asArray(item.ServiceTags)
};
