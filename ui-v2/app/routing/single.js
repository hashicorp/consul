import Route from '@ember/routing/route';
import { get } from '@ember/object';
import { assert } from '@ember/debug';
import { Promise, hash } from 'rsvp';
export default Route.extend({
  // repo: service('repositoryName'),
  isCreate: function(params, transition) {
    return transition.targetName.split('.').pop() === 'create';
  },
  model: function(params, transition) {
    const repo = get(this, 'repo');
    assert(
      "`repo` is undefined, please define RepositoryService using `repo: service('repositoryName')`",
      typeof repo !== 'undefined'
    );
    const dc = this.modelFor('dc').dc.Name;
    const create = this.isCreate(...arguments);
    return hash({
      isLoading: false,
      dc: dc,
      create: create,
      ...repo.status({
        item: create
          ? Promise.resolve(repo.create({ Datacenter: dc }))
          : repo.findBySlug(params.id, dc),
      }),
    });
  },
});
