export default (clickable, collection) =>
  (scope = '.popover-select') => {
    return {
      scope: scope,
      selected: clickable('button'),
      options: collection('li[role="none"]', {
        button: clickable('button'),
      }),
    };
  };
