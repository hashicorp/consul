import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';
import { get, action } from '@ember/object';
import isFolder from 'consul-ui/utils/isFolder';

export default class IndexRoute extends Route {
  @service('repository/kv') repo;

  queryParams = {
    sortBy: 'sort',
    kind: 'kind',
    search: {
      as: 'filter',
      replace: true,
    },
  };

  beforeModel() {
    // we are index or folder, so if the key doesn't have a trailing slash
    // add one to force a fake findBySlug
    const params = this.paramsFor(this.routeName);
    const key = params.key || '/';
    if (!isFolder(key)) {
      return this.replaceWith(this.routeName, key + '/');
    }
  }

  model(params) {
    let key = params.key || '/';
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.optionalParams().nspace;
    return hash({
      parent: this.repo.findBySlug({
        ns: nspace,
        dc: dc,
        id: key,
      }),
    }).then(model => {
      return hash({
        ...model,
        ...{
          items: this.repo
            .findAllBySlug({
              ns: nspace,
              dc: dc,
              id: get(model.parent, 'Key'),
            })
            .catch(e => {
              const status = get(e, 'errors.firstObject.status');
              switch (status) {
                case '403':
                  return this.transitionTo('dc.acls.tokens');
                default:
                  return this.transitionTo('dc.kv.index');
              }
            }),
        },
      });
    });
  }

  @action
  error(e) {
    if (e.errors && e.errors[0] && e.errors[0].status == '404') {
      return this.transitionTo('dc.kv.index');
    }
    // let the route above handle the error
    return true;
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
