export default (collection, clickable, attribute, text, actions) => () => {
  return collection('.consul-policy-list [data-test-list-row]', {
    name: attribute('data-test-policy', '[data-test-policy]'),
    description: text('[data-test-description]'),
    policy: clickable('a'),
    ...actions(['edit', 'delete']),
  });
};
