/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default (clickable, deletable, collection, alias, roleForm) =>
  (scope = '#roles') => {
    return {
      scope: scope,
      create: clickable('[data-test-role-create]'),
      form: roleForm(),
      roles: alias('selectedOptions'),
      selectedOptions: collection('[data-test-roles] [data-test-tabular-row]', {
        actions: clickable('label > button'),
        delete: clickable('[data-test-delete]'),
        confirmDelete: clickable('.informed-action button'),
      }),
    };
  };
