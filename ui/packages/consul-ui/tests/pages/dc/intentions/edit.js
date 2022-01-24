export default function(
  visitable,
  clickable,
  isVisible,
  submitable,
  deletable,
  cancelable,
  permissionsForm,
  permissionsList
) {
  return {
    scope: 'main',
    visit: visitable([
      '/:dc/intentions/:intention',
      '/:dc/services/:service/intentions/:intention',
      '/:dc/services/:service/intentions/create',
      '/:dc/intentions/create',
    ]),
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
      see: isVisible(),
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
