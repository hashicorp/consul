export default () => ({ kinds = [] }) => {
  const kindIncludes = ['global-management', 'global', 'local'].reduce((prev, item) => {
    prev[item] = kinds.includes(item);
    return prev;
  }, {});
  return item => {
    if (kinds.length > 0) {
      if (kindIncludes['global-management'] && item.isGlobalManagement) {
        return true;
      }
      if (kindIncludes['global'] && !item.Local) {
        return true;
      }
      if (kindIncludes['local'] && item.Local) {
        return true;
      }
      return false;
    }
    return true;
  };
};
