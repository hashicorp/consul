export default function(
  visitable,
  submitable,
  deletable,
  cancelable,
  clickable,
  policySelector,
  roleSelector
) {
  return {
    visit: visitable(['/:dc/acls/tokens/:token', '/:dc/acls/tokens/create']),
    ...submitable({}, 'form > div'),
    ...cancelable({}, 'form > div'),
    ...deletable({}, 'form > div'),
    use: clickable('[data-test-use]'),
    confirmUse: clickable('button.type-delete'),
    policies: policySelector(),
    roles: roleSelector(),
  };
}
