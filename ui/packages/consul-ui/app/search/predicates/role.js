/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default {
  Name: (item) => item.Name,
  Description: (item) => item.Description,
  Policy: (item) => {
    return (item.Policies || [])
      .map((item) => item.Name)
      .concat((item.ServiceIdentities || []).map((item) => item.ServiceName))
      .concat((item.NodeIdentities || []).map((item) => item.NodeName));
  },
};
