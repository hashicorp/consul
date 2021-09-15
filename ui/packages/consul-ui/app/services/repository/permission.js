import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { runInDebug } from '@ember/debug';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'permission';
// The set of permissions/resources required globally by the UI in order to
// run correctly
const REQUIRED_PERMISSIONS = [
  {
    Resource: 'operator',
    Access: 'write',
  },
  {
    Resource: 'operator',
    Access: 'read',
  },
  {
    Resource: 'service',
    Access: 'read',
  },
  {
    Resource: 'node',
    Access: 'read',
  },
  {
    Resource: 'session',
    Access: 'read',
  },
  {
    Resource: 'session',
    Access: 'write',
  },
  {
    Resource: 'key',
    Access: 'read',
  },
  {
    Resource: 'key',
    Access: 'write',
  },
  {
    Resource: 'intention',
    Access: 'read',
  },
  {
    Resource: 'intention',
    Access: 'write',
  },
  {
    Resource: 'acl',
    Access: 'read',
  },
  {
    Resource: 'acl',
    Access: 'write',
  },
];
export default class PermissionService extends RepositoryService {
  @service('env') env;
  @service('can') _can;

  // TODO: move this to the store, if we want it to use ember-data
  // currently this overwrites an inherited permissions service (this service)
  // which isn't ideal, but if the name of this changes be aware that we'd
  // probably have some circular dependency happening here
  @tracked permissions = [];

  getModelName() {
    return modelName;
  }

  has(permission) {
    const keys = Object.keys(permission);
    return this.permissions.some(item => {
      return keys.every(key => item[key] === permission[key]) && item.Allow === true;
    });
  }

  can(can) {
    return this._can.can(can);
  }

  abilityFor(str) {
    return this._can.abilityFor(str);
  }

  generate(resource, action, segment) {
    const req = {
      Resource: resource,
      Access: action,
    };
    if (typeof segment !== 'undefined') {
      req.Segment = segment;
    }
    return req;
  }

  /**
   * Requests the access for the defined resources/permissions from the backend.
   * If ACLs are disabled, then you have access to everything, hence we check
   * that here and only make the request if ACLs are enabled
   */
  async authorize(params) {
    if (!this.env.var('CONSUL_ACLS_ENABLED')) {
      return params.resources.map(item => {
        return {
          ...item,
          Allow: true,
        };
      });
    } else {
      let resources = [];
      try {
        resources = await this.store.authorize('permission', params);
      } catch (e) {
        runInDebug(() => console.error(e));
        // passthrough
      }
      return resources;
    }
  }

  async findBySlug(params, model) {
    let ability;
    try {
      ability = this._can.abilityFor(model);
    } catch (e) {
      return [];
    }

    const resources = ability.generateForSegment(params.id.toString());
    // if we get no resources for a segment it means that this
    // ability/permission isn't segmentable
    if (resources.length === 0) {
      return [];
    }
    params.resources = resources;
    return this.authorize(params);
  }

  async findByPermissions(params) {
    return this.authorize(params);
  }

  @dataSource('/:partition/:nspace/:dc/permissions')
  async findAll(params) {
    params.resources = REQUIRED_PERMISSIONS;
    this.permissions = await this.findByPermissions(params);
    return this.permissions;
  }
}
