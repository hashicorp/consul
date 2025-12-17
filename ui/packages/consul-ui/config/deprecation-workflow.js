/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable no-undef */
self.deprecationWorkflow = self.deprecationWorkflow || {};
self.deprecationWorkflow.config = {
  workflow: [
    { handler: 'throw', matchId: 'ember-views.partial' }, // CRITICAL
    { handler: 'throw', matchId: 'ember-component.send-action' }, // CRITICAL
    { handler: 'throw', matchId: 'computed-property.override' }, // CRITICAL
    { handler: 'throw', matchId: 'ember-cli-page-object.string-properties-on-definition' },
    { handler: 'throw', matchId: 'ember-sinon-qunit.test' },
    { handler: 'throw', matchId: 'ember-qunit.deprecate-legacy-apis' },
    { handler: 'throw', matchId: 'ember-can.can-service' },
    { handler: 'throw', matchId: 'ember-data:model.toJSON' },
    { handler: 'throw', matchId: 'ember-cli-page-object.is-property' },
    { handler: 'throw', matchId: 'ember-views.partial' },
    { handler: 'throw', matchId: 'ember-component.send-action' },
    { handler: 'throw', matchId: 'ember-cli-page-object.multiple' },
    { handler: 'throw', matchId: 'computed-property.override' },
    { handler: 'throw', matchId: 'autotracking.mutation-after-consumption' },
    { handler: 'throw', matchId: 'ember-data:legacy-test-helper-support' },
    { handler: 'throw', matchId: 'ember-data:Model.data' },
  ],
};
