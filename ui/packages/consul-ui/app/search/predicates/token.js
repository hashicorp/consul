/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  Name: (item) => item.Name,
  Description: (item) => item.Description,
  AccessorID: (item) => item.AccessorID,
  Role: (item) => (item.Roles || []).map((item) => item.Name),
  Policy: (item) => {
    return (item.Policies || [])
      .map((item) => item.Name)
      .concat((item.ServiceIdentities || []).map((item) => item.ServiceName))
      .concat((item.NodeIdentities || []).map((item) => item.NodeName));
  },
};
