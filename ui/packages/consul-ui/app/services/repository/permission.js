import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { runInDebug } from '@ember/debug';

const modelName = 'permission';
// The set of permissions/resources required globally by the UI in order to
// run correctly
const REQUIRED_PERMISSIONS = [
  {
    Resource: 'operator',
    Access: 'write',
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
  async authorize(resources, dc, nspace) {
    if (!this.env.var('CONSUL_ACLS_ENABLED')) {
      return resources.map(item => {
        return {
          ...item,
          Allow: true,
        };
      });
    } else {
      let permissions = [];
      try {
        permissions = await this.store.authorize('permission', {
          dc: dc,
          ns: nspace,
          permissions: resources,
        });
      } catch (e) {
        runInDebug(() => console.error(e));
        // passthrough
      }
      return permissions;
    }
  }

  async findBySlug(segment, model, dc, nspace) {
    let ability;
    try {
      ability = this._can.abilityFor(model);
    } catch (e) {
      return [];
    }

    const resources = ability.generateForSegment(segment.toString());
    // if we get no resources for a segment it means that this
    // ability/permission isn't segmentable
    if (resources.length === 0) {
      return [];
    }
    return this.authorize(resources, dc, nspace);
  }

  async findByPermissions(resources, dc, nspace) {
    return this.authorize(resources, dc, nspace);
  }

  async findAll(dc, nspace) {
    this.permissions = await this.findByPermissions(REQUIRED_PERMISSIONS, dc, nspace);
    return this.permissions;
  }
}
