export default {
  Name: (item, value) => item.Name.toLowerCase().indexOf(value.toLowerCase()) !== -1,
  Description: (item, value) => item.Description.toLowerCase().indexOf(value.toLowerCase()) !== -1,
  Role: (item, value) => {
    const acls = item.ACLs || {};
    return (acls.RoleDefaults || []).some(
      item => item.Name.toLowerCase().indexOf(value.toLowerCase()) !== -1
    );
  },
  Policy: (item, value) => {
    const acls = item.ACLs || {};
    return (acls.PolicyDefaults || []).some(
      item => item.Name.toLowerCase().indexOf(value.toLowerCase()) !== -1
    );
  },
};
