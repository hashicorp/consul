export default present => (scope = '.empty-state') => {
  return {
    scope: scope,
    login: present('[data-test-empty-state-login]'),
  };
};
