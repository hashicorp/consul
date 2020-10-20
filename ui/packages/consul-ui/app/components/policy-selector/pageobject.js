export default (clickable, deletable, collection, alias, policyForm) => (
  scope = '#policies',
  createSelector = '[for="new-policy-toggle"]'
) => {
  return {
    scope: scope,
    create: clickable(createSelector),
    form: policyForm('#new-policy-toggle + div'),
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
