import { set, get } from '@ember/object';
import RepositoryService from 'consul-ui/services/repository';
import { PRIMARY_KEY } from 'consul-ui/models/intention';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'intention';
export default class IntentionRepository extends RepositoryService {
  managedByCRDs = false;

  getModelName() {
    return modelName;
  }

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  create(obj) {
    delete obj.Namespace;
    return super.create({
      Action: 'allow',
      ...obj,
    });
  }

  isManagedByCRDs() {
    if (!this.managedByCRDs) {
      this.managedByCRDs = this.store
        .peekAll(this.getModelName())
        .toArray()
        .some(item => item.IsManagedByCRD);
    }
    return this.managedByCRDs;
  }

  // legacy intentions are strange that in order to read/write you need access
  // to either/or the destination or source
  async authorizeBySlug(cb, access, slug, dc, nspace) {
    const [, source, , destination] = slug.split(':');
    const ability = this.permissions.abilityFor(this.getModelName());
    const permissions = ability
      .generateForSegment(source)
      .concat(ability.generateForSegment(destination));
    return this.authorizeByPermissions(cb, permissions, access, dc, nspace);
  }

  async persist(obj) {
    const res = await super.persist(...arguments);
    // if Action is set it means we are an l4 type intention
    // we don't delete these at a UI level incase the user
    // would like to switch backwards and forwards between
    // allow/deny/l7 in the forms, but once its been saved
    // to the backend we then delete them
    if (get(res, 'Action.length')) {
      set(res, 'Permissions', []);
    }
    return res;
  }

  @dataSource('/:ns/:dc/intentions')
  async findAllByDatacenter() {
    return super.findAllByDatacenter(...arguments);
  }

  @dataSource('/:ns/:dc/intention/:id')
  async findBySlug() {
    return super.findBySlug(...arguments);
  }

  @dataSource('/:ns/:dc/intentions/for-service/:slug')
  async findByService(params, configuration = {}) {
    const query = {
      dc: params.dc,
      nspace: params.nspace,
      filter: `SourceName == "${params.slug}" or DestinationName == "${params.slug}" or SourceName == "*" or DestinationName == "*"`,
    };
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), query);
  }
}
