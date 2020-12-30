export default {
  Name: (item, value) => item.Name,
  Description: (item, value) => item.Description,
  Role: (item, value) => {
    const acls = item.ACLs || {};
    return (acls.RoleDefaults || []).map(item => item.Name);
  },
  Policy: (item, value) => {
    const acls = item.ACLs || {};
    return (acls.PolicyDefaults || []).map(item => item.Name);
  },
};
