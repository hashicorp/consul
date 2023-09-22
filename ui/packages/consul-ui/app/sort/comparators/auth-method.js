export default ({ properties }) =>
  (key = 'MethodName:asc') => {
    return properties(['MethodName', 'TokenTTL'])(key);
  };
