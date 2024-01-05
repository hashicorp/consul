/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  Name: (item) => item.Name,
  Description: (item) => item.Description,
  Role: (item) => {
    const acls = item.ACLs || {};
    return (acls.RoleDefaults || []).map((item) => item.Name);
  },
  Policy: (item) => {
    const acls = item.ACLs || {};
    return (acls.PolicyDefaults || []).map((item) => item.Name);
  },
};
