export default (submitable, cancelable, radiogroup) => () => {
  return {
    // this should probably be settable
    resetScope: true,
    scope: '[data-test-policy-form]',
    prefix: 'policy',
    ...submitable(),
    ...cancelable(),
    ...radiogroup('template', ['', 'service-identity'], 'policy'),
  };
};
