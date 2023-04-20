export default (submitable, clickable, attribute) =>
  (scope = '.auth-form') => {
    return {
      scope: scope,
      ...submitable(),
    };
  };
