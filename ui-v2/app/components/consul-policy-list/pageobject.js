export default (collection, clickable, attribute, text, actions) => () => {
  return collection('.consul-policy-list li:not(:first-child)', {
    name: attribute('data-test-policy', '[data-test-policy]'),
    description: text('[data-test-description]'),
    policy: clickable('a'),
    ...actions(['edit', 'delete']),
  });
};
