export default (collection, text) => () => {
  return collection('.consul-auth-method-list [data-test-list-row]', {
    name: text('[data-test-auth-method]'),
    displayName: text('[data-test-display-name]'),
    type: text('[data-test-type]'),
  });
};
