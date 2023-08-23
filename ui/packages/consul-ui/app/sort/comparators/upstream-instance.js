export default ({ properties }) => (key = 'DestinationName:asc') => {
  return properties(['DestinationName'])(key);
};
