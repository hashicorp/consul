export default function(
  visitable,
  clickable,
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
    ...submitable(),
    ...cancelable(),
    ...deletable(),
  };
}
