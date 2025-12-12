/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

/* eslint-disable no-undef */
self.deprecationWorkflow = self.deprecationWorkflow || {};
self.deprecationWorkflow.config = {
  workflow: [
    { handler: 'log', matchId: 'ember-cli-page-object.string-properties-on-definition' },
    { handler: 'log', matchId: 'ember-sinon-qunit.test' },
    { handler: 'log', matchId: 'ember-qunit.deprecate-legacy-apis' },
    { handler: 'log', matchId: 'ember-can.can-service' },
    { handler: 'log', matchId: 'ember-data:model.toJSON' },
    { handler: 'log', matchId: 'ember-cli-page-object.is-property' },
    { handler: 'log', matchId: 'ember-views.partial' },
    { handler: 'log', matchId: 'ember-component.send-action' },
    { handler: 'log', matchId: 'ember-cli-page-object.multiple' },
    { handler: 'log', matchId: 'computed-property.override' },
    { handler: 'log', matchId: 'autotracking.mutation-after-consumption' },
    { handler: 'log', matchId: 'ember-data:legacy-test-helper-support' },
    { handler: 'log', matchId: 'ember-data:Model.data' },
  ],
};
