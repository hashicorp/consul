export default (submitable, cancelable, radiogroup, text) => (
  scope = '[data-test-policy-form]'
) => {
  return {
    // this should probably be settable
    resetScope: true,
    scope: scope,
    prefix: 'policy',
    ...submitable(),
    ...cancelable(),
    ...radiogroup('template', ['', 'service-identity', 'node-identity'], 'policy'),
    rules: {
      error: text('[data-test-rules] strong'),
    },
  };
};
