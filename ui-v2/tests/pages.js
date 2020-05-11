import {
  create,
  clickable,
  is,
  attribute,
  collection,
  text,
  isPresent,
} from 'ember-cli-page-object';
import { alias } from 'ember-cli-page-object/macros';
import { visitable } from 'consul-ui/tests/lib/page-object/visitable';
import createDeletable from 'consul-ui/tests/lib/page-object/createDeletable';
import createSubmitable from 'consul-ui/tests/lib/page-object/createSubmitable';
import createCreatable from 'consul-ui/tests/lib/page-object/createCreatable';
import createCancelable from 'consul-ui/tests/lib/page-object/createCancelable';

// TODO: All component-like page objects should be moved into the component folder
// along with all of its other dependencies once we can mae ember-cli ignore them
import page from 'consul-ui/tests/pages/components/page';
import radiogroup from 'consul-ui/tests/lib/page-object/radiogroup';
import tabgroup from 'consul-ui/tests/lib/page-object/tabgroup';
import freetextFilter from 'consul-ui/tests/pages/components/freetext-filter';
import catalogFilter from 'consul-ui/tests/pages/components/catalog-filter';
import catalogToolbar from 'consul-ui/tests/pages/components/catalog-toolbar';
import popoverSort from 'consul-ui/tests/pages/components/popover-sort';
import aclFilter from 'consul-ui/tests/pages/components/acl-filter';
import intentionFilter from 'consul-ui/tests/pages/components/intention-filter';
import tokenListFactory from 'consul-ui/tests/pages/components/token-list';
import policyFormFactory from 'consul-ui/tests/pages/components/policy-form';
import policySelectorFactory from 'consul-ui/tests/pages/components/policy-selector';
import roleFormFactory from 'consul-ui/tests/pages/components/role-form';
import roleSelectorFactory from 'consul-ui/tests/pages/components/role-selector';
import consulIntentionListFactory from 'consul-ui/tests/pages/components/consul-intention-list';

// TODO: should this specifically be modal or form?
// should all forms be forms?

import index from 'consul-ui/tests/pages/index';
import dcs from 'consul-ui/tests/pages/dc';
import settings from 'consul-ui/tests/pages/settings';
import services from 'consul-ui/tests/pages/dc/services/index';
import service from 'consul-ui/tests/pages/dc/services/show';
import instance from 'consul-ui/tests/pages/dc/services/instance';
import nodes from 'consul-ui/tests/pages/dc/nodes/index';
import node from 'consul-ui/tests/pages/dc/nodes/show';
import kvs from 'consul-ui/tests/pages/dc/kv/index';
import kv from 'consul-ui/tests/pages/dc/kv/edit';
import acls from 'consul-ui/tests/pages/dc/acls/index';
import acl from 'consul-ui/tests/pages/dc/acls/edit';
import policies from 'consul-ui/tests/pages/dc/acls/policies/index';
import policy from 'consul-ui/tests/pages/dc/acls/policies/edit';
import roles from 'consul-ui/tests/pages/dc/acls/roles/index';
import role from 'consul-ui/tests/pages/dc/acls/roles/edit';
import tokens from 'consul-ui/tests/pages/dc/acls/tokens/index';
import token from 'consul-ui/tests/pages/dc/acls/tokens/edit';
import intentions from 'consul-ui/tests/pages/dc/intentions/index';
import intention from 'consul-ui/tests/pages/dc/intentions/edit';
import nspaces from 'consul-ui/tests/pages/dc/nspaces/index';
import nspace from 'consul-ui/tests/pages/dc/nspaces/edit';

const deletable = createDeletable(clickable);
const submitable = createSubmitable(clickable, is);
const creatable = createCreatable(clickable, is);
const cancelable = createCancelable(clickable, is);

const tokenList = tokenListFactory(clickable, attribute, collection, deletable);

const policyForm = policyFormFactory(submitable, cancelable, radiogroup, text);
const policySelector = policySelectorFactory(clickable, deletable, collection, alias, policyForm);

const roleForm = roleFormFactory(submitable, cancelable, policySelector);
const roleSelector = roleSelectorFactory(clickable, deletable, collection, alias, roleForm);

const consulIntentionList = consulIntentionListFactory(collection, clickable, attribute, deletable);

export default {
  index: create(index(visitable, collection)),
  dcs: create(dcs(visitable, clickable, attribute, collection)),
  services: create(
    services(visitable, clickable, text, attribute, collection, page, popoverSort, radiogroup)
  ),
  service: create(
    service(visitable, attribute, collection, text, consulIntentionList, catalogToolbar, tabgroup)
  ),
  instance: create(instance(visitable, attribute, collection, text, tabgroup)),
  nodes: create(nodes(visitable, clickable, attribute, collection, catalogFilter)),
  node: create(node(visitable, deletable, clickable, attribute, collection, tabgroup)),
  kvs: create(kvs(visitable, deletable, creatable, clickable, attribute, collection)),
  kv: create(kv(visitable, attribute, submitable, deletable, cancelable, clickable)),
  acls: create(acls(visitable, deletable, creatable, clickable, attribute, collection, aclFilter)),
  acl: create(acl(visitable, submitable, deletable, cancelable, clickable)),
  policies: create(
    policies(
      visitable,
      deletable,
      creatable,
      clickable,
      attribute,
      collection,
      text,
      freetextFilter
    )
  ),
  policy: create(policy(visitable, submitable, deletable, cancelable, clickable, tokenList)),
  roles: create(
    roles(visitable, deletable, creatable, clickable, attribute, collection, text, freetextFilter)
  ),
  // TODO: This needs a policyList
  role: create(role(visitable, submitable, deletable, cancelable, policySelector, tokenList)),
  tokens: create(
    tokens(
      visitable,
      submitable,
      deletable,
      creatable,
      clickable,
      attribute,
      collection,
      text,
      freetextFilter
    )
  ),
  token: create(
    token(visitable, submitable, deletable, cancelable, clickable, policySelector, roleSelector)
  ),
  intentions: create(intentions(visitable, creatable, consulIntentionList, intentionFilter)),
  intention: create(intention(visitable, submitable, deletable, cancelable)),
  nspaces: create(
    nspaces(visitable, deletable, creatable, clickable, attribute, collection, text, freetextFilter)
  ),
  nspace: create(
    nspace(visitable, submitable, deletable, cancelable, policySelector, roleSelector)
  ),
  settings: create(settings(visitable, submitable, isPresent)),
};
