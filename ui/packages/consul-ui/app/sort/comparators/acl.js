export default ({ properties }) => (key = 'Name:asc') => {
  return properties(['Name'])(key);
};
