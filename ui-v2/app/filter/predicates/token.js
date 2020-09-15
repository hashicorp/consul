export default () => ({ types = [] }) => {
  const typeIncludes = ['global-management', 'global', 'local'].reduce((prev, item) => {
    prev[item] = types.includes(item);
    return prev;
  }, {});
  return item => {
    if (types.length > 0) {
      if (typeIncludes['global-management'] && item.isGlobalManagement) {
        return true;
      }
      if (typeIncludes['global'] && !item.Local) {
        return true;
      }
      if (typeIncludes['local'] && item.Local) {
        return true;
      }
      return false;
    }
    return true;
  };
};
