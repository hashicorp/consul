export default (clickable, deletable, collection, alias, roleForm) => (scope = '#roles') => {
  return {
    scope: scope,
    create: clickable('[for="new-role-toggle"]'),
    form: roleForm(),
    roles: alias('selectedOptions'),
    selectedOptions: collection(
      '[data-test-roles] [data-test-tabular-row]',
      deletable({
        actions: clickable('label'),
      })
    ),
  };
};
