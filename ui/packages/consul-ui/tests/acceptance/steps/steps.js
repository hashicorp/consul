import steps from 'consul-ui/tests/steps';
import pages from 'consul-ui/tests/pages';
import Inflector from 'ember-inflector';
import utils from '@ember/test-helpers';
import $ from '-jquery';

import api from 'consul-ui/tests/helpers/api';

export default function({ assert, library }) {
  return steps({
    assert,
    library,
    pages,
    utils,
    api,
    Inflector,
    $,
  });
}
