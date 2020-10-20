export default () => ({ statuses = [] }) => {
  return item => {
    if (statuses.length > 0 && !statuses.includes(item.Status)) {
      return false;
    }
    return true;
  };
};
