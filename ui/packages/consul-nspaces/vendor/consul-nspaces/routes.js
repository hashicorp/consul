(routes => routes({
  dc: {
    nspaces: {
      _options: {
        path: '/namespaces',
        queryParams: {
          sortBy: 'sort',
          searchproperty: {
            as: 'searchproperty',
            empty: [['Name', 'Description', 'Role', 'Policy']],
          },
          search: {
            as: 'filter',
            replace: true,
          },
        },
        abilities: ['read nspaces'],
      },
      edit: {
        _options: { path: '/:name' },
      },
      create: {
        _options: {
          template: 'dc/nspaces/edit',
          path: '/create',
          abilities: ['create nspaces'],
        },
      },
    },
  },
}))(
  (json, data = document.currentScript.dataset) => {
    const appNameJS = data.appName.split('-')
      .map((item, i) => i ? `${item.substr(0, 1).toUpperCase()}${item.substr(1)}` : item)
      .join('');
    data[`${appNameJS}Routes`] = JSON.stringify(json);
  }
);
