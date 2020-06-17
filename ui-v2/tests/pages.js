import {
  create as createPage,
  clickable,
  is,
  attribute,
  collection,
  text,
  isPresent,
  triggerable,
} from 'ember-cli-page-object';

import { alias } from 'ember-cli-page-object/macros';
import { visitable } from 'consul-ui/tests/lib/page-object/visitable';

// utils
import createDeletable from 'consul-ui/tests/lib/page-object/createDeletable';
import createSubmitable from 'consul-ui/tests/lib/page-object/createSubmitable';
import createCreatable from 'consul-ui/tests/lib/page-object/createCreatable';
import createCancelable from 'consul-ui/tests/lib/page-object/createCancelable';

// components
import pageFactory from 'consul-ui/components/hashicorp-consul/pageobject';

import radiogroup from 'consul-ui/components/radio-group/pageobject';
import tabgroup from 'consul-ui/components/tab-nav/pageobject';
import popoverSort from 'consul-ui/components/popover-sort/pageobject';
import authFormFactory from 'consul-ui/components/auth-form/pageobject';
import freetextFilterFactory from 'consul-ui/components/freetext-filter/pageobject';

import searchBarFactory from 'consul-ui/components/search-bar/pageobject';

import policyFormFactory from 'consul-ui/components/policy-form/pageobject';
import policySelectorFactory from 'consul-ui/components/policy-selector/pageobject';
import roleFormFactory from 'consul-ui/components/role-form/pageobject';
import roleSelectorFactory from 'consul-ui/components/role-selector/pageobject';
import tokenListFactory from 'consul-ui/components/token-list/pageobject';
import consulTokenListFactory from 'consul-ui/components/consul-token-list/pageobject';
import consulIntentionListFactory from 'consul-ui/components/consul-intention-list/pageobject';

// pages
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

// utils
const deletable = createDeletable(clickable);
const submitable = createSubmitable(clickable, is);
const creatable = createCreatable(clickable, is);
const cancelable = createCancelable(clickable, is);

// components
const tokenList = tokenListFactory(clickable, attribute, collection, deletable);
const authForm = authFormFactory(submitable, clickable, attribute);
const freetextFilter = freetextFilterFactory(triggerable);
const catalogToolbar = searchBarFactory(freetextFilter);
const catalogFilter = searchBarFactory(freetextFilter, () =>
  radiogroup('status', ['', 'passing', 'warning', 'critical'])
);
const aclFilter = searchBarFactory(freetextFilter, () =>
  radiogroup('type', ['', 'management', 'client'])
);
const intentionFilter = searchBarFactory(freetextFilter, () =>
  radiogroup('currentFilter', ['', 'allow', 'deny'])
);

const policyForm = policyFormFactory(submitable, cancelable, radiogroup, text);
const policySelector = policySelectorFactory(clickable, deletable, collection, alias, policyForm);
const roleForm = roleFormFactory(submitable, cancelable, policySelector);
const roleSelector = roleSelectorFactory(clickable, deletable, collection, alias, roleForm);
const consulIntentionList = consulIntentionListFactory(collection, clickable, attribute, deletable);
const consulTokenList = consulTokenListFactory(collection, clickable, attribute, text, deletable);

const page = pageFactory(clickable, attribute, is, authForm);

// pages
const create = function(appView) {
  appView = {
    ...page(),
    ...appView,
  };
  return createPage(appView);
};
export default {
  index: create(index(visitable, collection)),
  dcs: create(dcs(visitable, clickable, attribute, collection)),
  services: create(
    services(visitable, clickable, text, attribute, isPresent, collection, popoverSort, radiogroup)
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
  tokens: create(tokens(visitable, creatable, text, consulTokenList, freetextFilter)),
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
