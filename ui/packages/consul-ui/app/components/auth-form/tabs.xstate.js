export default {
  id: 'auth-form-tabs',
  initial: 'token',
  on: {
    TOKEN: [
      {
        target: 'token',
      },
    ],
    SSO: [
      {
        target: 'sso',
      },
    ],
  },
  states: {
    token: {},
    sso: {},
  },
};
