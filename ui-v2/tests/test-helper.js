import Application from '../app';
import config from '../config/environment';
import { setApplication } from '@ember/test-helpers';
import './helpers/flash-message';
import start from 'ember-exam/test-support/start';

const application = Application.create(config.APP);
application.inject('component:copy-button', 'clipboard', 'service:clipboard/local-storage');
setApplication(application);

start();
