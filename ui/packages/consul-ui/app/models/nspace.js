import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export const PRIMARY_KEY = 'Name';
// keep this for consistency
export const SLUG_KEY = 'Name';
export const NSPACE_KEY = 'Namespace';
export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),

  Description: attr('string', { defaultValue: '' }),
  // TODO: Is there some sort of date we can use here
  DeletedAt: attr('string'),
  ACLs: attr(undefined, function() {
    return { defaultValue: { PolicyDefaults: [], RoleDefaults: [] } };
  }),

  SyncTime: attr('number'),
});
