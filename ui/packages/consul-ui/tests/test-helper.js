/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Application from 'consul-ui/app';
import config from 'consul-ui/config/environment';
import * as QUnit from 'qunit';
import { setApplication } from '@ember/test-helpers';
import { setup } from 'qunit-dom';
import './helpers/flash-message';
import { setupEmberOnerrorValidation } from 'ember-qunit';
import { start as startEmberExam } from 'ember-exam/test-support';
import setupSinon from 'ember-sinon-qunit';
import { buildWaiter } from 'ember-test-waiters';

import ClientConnections from 'consul-ui/services/client/connections';

const waiter = buildWaiter('client-connections');
let tokens = [];

ClientConnections.reopen({
  addVisibilityChange: function () {
    // for the moment don't listen for tab hiding during testing
    // TODO: make this controllable from testing so we can fake a tab hide
  },
  purge: function () {
    const res = this._super(...arguments);
    while (tokens.length) {
      waiter.endAsync(tokens.pop());
    }
    return res;
  },
  acquire: function () {
    tokens.push(waiter.beginAsync());
    return this._super(...arguments);
  },
  release: function () {
    const res = this._super(...arguments);
    const t = tokens.pop();
    if (t) waiter.endAsync(t);
    return res;
  },
});
const application = Application.create(config.APP);

setApplication(application);

setup(QUnit.assert);
setupEmberOnerrorValidation(QUnit);
setupSinon();
startEmberExam();
