export default function (
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
    ...submitable({}, 'main form > div'),
    ...cancelable({}, 'main form > div'),
    ...deletable({}, 'main form > div'),
    use: clickable('[data-test-use]'),
    confirmUse: clickable('button.type-delete'),
    clone: clickable('[data-test-clone]'),
    policies: policySelector(),
    roles: roleSelector(),
  };
}
