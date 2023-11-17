import Application from 'consul-ui/app';
import config from 'consul-ui/config/environment';
import * as QUnit from 'qunit';
import { setApplication } from '@ember/test-helpers';
import { setup } from 'qunit-dom';
import { registerWaiter } from '@ember/test';
import './helpers/flash-message';
import start from 'ember-exam/test-support/start';
import setupSinon from 'ember-sinon-qunit';

import ClientConnections from 'consul-ui/services/client/connections';

let activeRequests = 0;
registerWaiter(function () {
  return activeRequests === 0;
});
ClientConnections.reopen({
  addVisibilityChange: function () {
    // for the moment don't listen for tab hiding during testing
    // TODO: make this controllable from testing so we can fake a tab hide
  },
  purge: function () {
    const res = this._super(...arguments);
    activeRequests = 0;
    return res;
  },
  acquire: function () {
    activeRequests++;
    return this._super(...arguments);
  },
  release: function () {
    const res = this._super(...arguments);
    activeRequests--;
    return res;
  },
});
const application = Application.create(config.APP);
application.inject('component:copy-button', 'clipboard', 'service:clipboard/local-storage');

setApplication(application);

setup(QUnit.assert);
setupSinon();

setup(QUnit.assert);

start();
