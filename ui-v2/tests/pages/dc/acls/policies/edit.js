export default function(visitable, submitable, deletable, cancelable, tokenList) {
  return {
    visit: visitable(['/:dc/acls/policies/:policy', '/:dc/acls/policies/create']),
    ...submitable({}, 'form > div'),
    ...cancelable({}, 'form > div'),
    ...deletable({}, 'form > div'),
    tokens: tokenList(),
    deleteModal: {
      resetScope: true,
      scope: '[data-test-delete-modal]',
      ...deletable({}),
    },
  };
}
