import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import ascend from 'consul-ui/utils/ascend';

export default class EditRoute extends Route {
  @service('repository/kv') repo;
  @service('repository/session') sessionRepo;
  @service('repository/permission') permissions;

  model(params) {
    const create =
      this.routeName
        .split('.')
        .pop()
        .indexOf('create') !== -1;
    const key = params.key;
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.optionalParams().nspace;
    return hash({
      dc: dc,
      nspace: nspace,
      parent:
        typeof key !== 'undefined'
          ? this.repo.findBySlug({
              ns: nspace,
              dc: dc,
              id: ascend(key, 1) || '/',
            })
          : this.repo.findBySlug({
              ns: nspace,
              dc: dc,
              id: '/',
            }),
      item: create
        ? this.repo.create({
            Datacenter: dc,
            Namespace: nspace,
          })
        : this.repo.findBySlug({
            ns: nspace,
            dc: dc,
            id: key,
          }),
      session: null,
    }).then(model => {
      // TODO: Consider loading this after initial page load
      if (typeof model.item !== 'undefined') {
        const session = get(model.item, 'Session');
        if (session && this.permissions.can('read sessions')) {
          return hash({
            ...model,
            ...{
              session: this.sessionRepo.findByKey({
                ns: nspace,
                dc: dc,
                id: session,
              }),
            },
          });
        }
      }
      return model;
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
