export default function(
  visitable,
  clickable,
  isPresent,
  submitable,
  deletable,
  cancelable,
  permissionsForm,
  permissionsList
) {
  return {
    scope: 'main',
    visit: visitable(['/:dc/intentions/:intention', '/:dc/intentions/create']),
    permissions: {
      create: {
        scope: '[data-test-create-permission]',
        click: clickable(),
      },
      form: permissionsForm(),
      list: permissionsList(),
    },
    warning: {
      scope: '[data-test-action-warning]',
      resetScope: true,
      present: isPresent(),
      confirm: {
        scope: '[data-test-action-warning-confirm]',
        click: clickable(),
      },
      cancel: {
        scope: '[data-test-action-warning-cancel]',
        click: clickable(),
      },
    },
    ...submitable(),
    ...cancelable(),
    ...deletable(),
  };
}
