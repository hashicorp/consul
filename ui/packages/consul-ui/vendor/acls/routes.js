(function(appNameJS = 'consulUi', doc = document) {
  const scripts = doc.getElementsByTagName('script');
  const script = scripts[scripts.length - 1];
  script.dataset[`${appNameJS}Routes`] = JSON.stringify({
    dc: {
      acls: {
        tokens: {
          _options: {
            abilities: ['read tokens'],
          },
        },
      },
    },
  });
})();
