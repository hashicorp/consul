export default ({ properties }) => (key = 'Name:asc') => {
  return properties(['Name', 'MaxTokenTTL'])(key);
};
