export default ({ properties }) => key => {
  return properties(['CreatedDate'])(key);
};
