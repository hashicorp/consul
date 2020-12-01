import Route from './services';

export default class UpstreamsRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    instance: 'instance',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'Tags']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };
  templateName = 'dc/services/show/upstreams';
}
