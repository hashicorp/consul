export default function(visitable, submitable, deletable, cancelable, clickable, tokenList) {
  return {
    visit: visitable(['/:dc/acls/policies/:policy', '/:dc/acls/policies/create']),
    ...submitable({}, 'main form > div'),
    ...cancelable({}, 'main form > div'),
    ...deletable({}, 'main form > div'),
    tokens: tokenList(),
    validDatacenters: clickable('[name="policy[isScoped]"]'),
    datacenter: clickable('[name="policy[Datacenters]"]'),
    deleteModal: {
      resetScope: true,
      scope: '[data-test-delete-modal]',
      ...deletable({}),
    },
  };
}
