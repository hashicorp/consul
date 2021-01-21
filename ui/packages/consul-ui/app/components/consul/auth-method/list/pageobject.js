export default (collection, attribute, text) => () => {
  return collection('.consul-auth-method-list [data-test-list-row]', {
    name: attribute('data-test-auth-method', '[data-test-auth-method]'),
    displayName: text('[data-test-display-name]'),
    type: text('[data-test-type]'),
  });
};
