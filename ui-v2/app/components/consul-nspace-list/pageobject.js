export default (collection, clickable, attribute, text, actions) => () => {
  return collection('.consul-nspace-list li:not(:first-child)', {
    nspace: clickable('a'),
    description: text('[data-test-description]'),
    ...actions(['edit', 'delete']),
  });
};
