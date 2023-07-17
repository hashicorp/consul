export default {
  kind: {
    'global-management': (item, value) => item.isGlobalManagement,
    global: (item, value) => !item.Local,
    local: (item, value) => item.Local,
  },
};
