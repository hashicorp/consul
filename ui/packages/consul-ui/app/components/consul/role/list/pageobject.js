export default (collection, clickable, attribute, text, actions) => () => {
  return collection('.consul-role-list [data-test-list-row]', {
    role: clickable('a'),
    name: attribute('data-test-role', '[data-test-role]'),
    description: text('[data-test-description]'),
    policy: text('[data-test-policy].policy', { multiple: true }),
    serviceIdentity: text('[data-test-policy].policy-service-identity', { multiple: true }),
    ...actions(['edit', 'delete']),
  });
};
