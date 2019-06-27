import Application from '../app';
import config from '../config/environment';
import { setApplication } from '@ember/test-helpers';
import { start } from 'ember-qunit';
import './helpers/flash-message';
import loadEmberExam from 'ember-exam/test-support/load';

loadEmberExam();
const application = Application.create(config.APP);
application.inject('component:copy-button', 'clipboard', 'service:clipboard/local-storage');
setApplication(application);

start();
