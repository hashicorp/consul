export default () => ({ accesses = [] }) => item => {
  if (accesses.length > 0) {
    if (accesses.includes(item.Action)) {
      return true;
    }
    return false;
  }
  return true;
};
