export default (collection, clickable, attribute, text, deletable) => () => {
  return collection('.consul-token-list li:not(:first-child)', {
    id: attribute('data-test-token', '[data-test-token]'),
    description: text('[data-test-description]'),
    policy: text('[data-test-policy].policy', { multiple: true }),
    role: text('[data-test-policy].role', { multiple: true }),
    serviceIdentity: text('[data-test-policy].policy-service-identity', { multiple: true }),
    token: clickable('a'),
    actions: clickable('label'),
    use: clickable('[data-test-use]'),
    confirmUse: clickable('[data-test-confirm-use]'),
    clone: clickable('[data-test-clone]'),
    ...deletable(),
  });
};
