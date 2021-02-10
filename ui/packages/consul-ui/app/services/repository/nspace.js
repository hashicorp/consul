import RepositoryService from 'consul-ui/services/repository';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/nspace';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'nspace';
export default class NspaceService extends RepositoryService {
  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  getSlugKey() {
    return SLUG_KEY;
  }

  getModelName() {
    return modelName;
  }

  remove(item) {
    // Namespace deletion is more of a soft delete.
    // Therefore the namespace still exists once we've requested a delete/removal.
    // This makes 'removing' more of a custom action rather than a standard
    // ember-data delete.
    // Here we use the same request for a delete but we bypass ember-data's
    // destroyRecord/unloadRecord and serialization so we don't get
    // ember data error messages when the UI tries to update a 'DeletedAt' property
    // on an object that ember-data is trying to delete
    const res = this.store.adapterFor('nspace').rpc(
      (adapter, request, serialized, unserialized) => {
        return adapter.requestForDeleteRecord(request, serialized, unserialized);
      },
      (serializer, respond, serialized, unserialized) => {
        return item;
      },
      item,
      'nspace'
    );
    return res;
  }

  @dataSource('/:ns/:dc/namespaces')
  findAll(params, configuration = {}) {
    const query = {};
    if (typeof configuration.cursor !== 'undefined') {
      query.index = configuration.cursor;
      query.uri = configuration.uri;
    }
    return this.store.query(this.getModelName(), query);
  }
}
