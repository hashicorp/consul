export default (collection, clickable, text) => () => {
  return collection('.consul-auth-method-list [data-test-list-row]', {
    authMethod: clickable('a'),
    name: text('[data-test-auth-method]'),
    displayName: text('[data-test-display-name]'),
    type: text('[data-test-type]'),
  });
};
