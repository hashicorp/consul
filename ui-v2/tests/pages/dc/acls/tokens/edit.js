import { alias } from 'ember-cli-page-object/macros';
export default function(
  visitable,
  submitable,
  deletable,
  cancelable,
  clickable,
  attribute,
  collection
) {
  const policySelector = function(
    scope = '#policies',
    createSelector = '[for="new-policy-toggle"]'
  ) {
    return {
      scope: scope,
      create: clickable(createSelector),
      form: {
        resetScope: true,
        scope: '[data-test-policy-form]',
        prefix: 'policy',
        ...submitable(),
        ...cancelable(),
      },
      policies: alias('selectedOptions'),
      selectedOptions: collection(
        '[data-test-policies] [data-test-tabular-row]',
        deletable(
          {
            expand: clickable('label'),
          },
          '+ tr'
        )
      ),
    };
  };
  const roleSelector = function(scope = '#roles') {
    return {
      scope: scope,
      create: clickable('[for="new-role-toggle"]'),
      form: {
        resetScope: true,
        scope: '[data-test-role-form]',
        prefix: 'role',
        ...submitable(),
        ...cancelable(),
        policies: policySelector('', '[data-test-create-policy]'),
      },
      roles: alias('selectedOptions'),
      selectedOptions: collection(
        '[data-test-roles] [data-test-tabular-row]',
        deletable({
          actions: clickable('label'),
        })
      ),
    };
  };
  return {
    visit: visitable(['/:dc/acls/tokens/:token', '/:dc/acls/tokens/create']),
    ...submitable({}, 'form > div'),
    ...cancelable({}, 'form > div'),
    ...deletable({}, 'form > div'),
    use: clickable('[data-test-use]'),
    confirmUse: clickable('button.type-delete'),
    // TODO: Also see tokens/edit, these should get injected
    policies: policySelector(),
    roles: roleSelector(),
  };
}
