/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import Service, { inject as service } from '@ember/service';
import DataSourceComponent from 'consul-ui/components/data-source/index';
import sinon from 'sinon';
import { BlockingEventSource as RealEventSource } from 'consul-ui/utils/dom/event-source';
import { ACCESS_LEVEL } from 'consul-ui/components/link-to-hcp-modal';

const modalSelector = '[data-test-link-to-hcp-modal]';
const modalNoACLsAlertSelector = '[data-test-link-to-hcp-modal-no-acls-alert]';
const modalOptionReadOnlySelector = '#accessMode-readonly';
const modalOptionReadOnlyErrorSelector = '[data-test-link-to-hcp-modal-access-level-options-error]';
const modalGenerateTokenCardSelector = '[data-test-link-to-hcp-modal-generate-token-card]';
const modalGenerateTokenCardValueSelector =
  '[data-test-link-to-hcp-modal-generate-token-card-value]';
const modalGenerateTokenCardCopyButtonSelector =
  '[data-test-link-to-hcp-modal-generate-token-card-copy-button]';
const modalGenerateTokenButtonSelector = '[data-test-link-to-hcp-modal-generate-token-button]';
const modalGenerateTokenMissedPolicyAlertSelector =
  '[data-test-link-to-hcp-modal-missed-policy-alert]';
const modalNextButtonSelector = '[data-test-link-to-hcp-modal-next-button]';
const modalCancelButtonSelector = '[data-test-link-to-hcp-modal-cancel-button]';

module('Integration | Component | link-to-hcp-modal', function (hooks) {
  let originalClipboardWriteText;
  let hideModal = sinon.stub();
  const close = sinon.stub();
  const source = new RealEventSource();

  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    const fakeService = class extends Service {
      close = close;
      open() {
        source.getCurrentEvent = function () {
          return { data: { Name: 'global-read-only', ID: '00000000-0000-0000-0000-000000000002' } };
        };
        return source;
      }
    };
    this.owner.register('service:data-source/fake-service', fakeService);
    this.owner.register(
      'component:data-source',
      class extends DataSourceComponent {
        @service('data-source/fake-service') dataSource;
      }
    );
    this.owner.register(
      'service:abilities',
      class Stub extends Service {
        can(permission) {
          if (permission === 'create tokens') {
            return true;
          }
          if (permission === 'read acls') {
            return true;
          }
        }
      }
    );
    this.owner.register(
      'service:hcp-link-modal',
      class Stub extends Service {
        hide = hideModal;
      }
    );

    originalClipboardWriteText = navigator.clipboard.writeText;
    navigator.clipboard.writeText = sinon.stub();
  });

  hooks.afterEach(function () {
    navigator.clipboard.writeText = originalClipboardWriteText;
  });

  test('it renders modal', async function (assert) {
    await render(hbs`<LinkToHcpModal @dc="dc-1"
                                     @nspace="default"
                                     @partition="-" />`);

    assert.dom(modalSelector).exists({ count: 1 });
    assert.dom(`${modalSelector} ${modalNoACLsAlertSelector}`).doesNotExist();

    // select read-only
    await click(`${modalSelector} ${modalOptionReadOnlySelector}`);

    // when read-only selected, it shows the generate token button
    assert.dom(`${modalSelector} ${modalGenerateTokenButtonSelector}`).isVisible();

    // with the correct policy, it doesn't show the missed policy alert
    assert.dom(`${modalSelector} ${modalGenerateTokenMissedPolicyAlertSelector}`).doesNotExist();
  });

  test('it updates next link on option selected', async function (assert) {
    await render(hbs`<LinkToHcpModal @dc="dc-1"
                                     @nspace="default"
                                     @partition="-" />`);

    let hrefValue = this.element
      .querySelector(`${modalSelector} ${modalNextButtonSelector}`)
      .getAttribute('href');
    assert.ok(
      hrefValue.includes(ACCESS_LEVEL.GLOBALREADWRITE),
      'next link includes read/write access level'
    );

    // select read-only
    await click(`${modalSelector} ${modalOptionReadOnlySelector}`);

    hrefValue = this.element
      .querySelector(`${modalSelector} ${modalNextButtonSelector}`)
      .getAttribute('href');
    assert.ok(
      hrefValue.includes(ACCESS_LEVEL.GLOBALREADONLY),
      'next link includes read-only access level'
    );
  });

  test('it creates token and copy it to clipboard', async function (assert) {
    await render(hbs`<LinkToHcpModal @dc="dc-1"
                            @nspace="default"
                            @partition="-" />`);
    // select read-only
    await click(`${modalSelector} ${modalOptionReadOnlySelector}`);
    assert
      .dom(`${modalSelector} ${modalGenerateTokenButtonSelector}`)
      .hasText('Generate a read-only ACL token');

    // with the correct policy, it doesn't show the missed policy alert
    assert.dom(`${modalSelector} ${modalGenerateTokenMissedPolicyAlertSelector}`).doesNotExist();

    // trigger generate token
    await click(`${modalSelector} ${modalGenerateTokenButtonSelector}`);

    assert.dom(`${modalSelector} ${modalGenerateTokenCardSelector}`).isVisible();
    assert.dom(`${modalSelector} ${modalGenerateTokenCardValueSelector}`).exists();
    const tokenValue = this.element.querySelector(
      `${modalSelector} ${modalGenerateTokenCardValueSelector}`
    ).textContent;
    // click on copy button
    await click(`${modalSelector} ${modalGenerateTokenCardCopyButtonSelector}`);
    assert.ok(
      navigator.clipboard.writeText.called,
      'clipboard write function is called when copy button is clicked'
    );
    assert.ok(
      navigator.clipboard.writeText.calledWith(tokenValue.trim()),
      'clipboard contains expected value'
    );
  });

  test('it calls hcpLinkModal.hide when closing modal', async function (assert) {
    await render(hbs`<LinkToHcpModal @dc="dc-1"
                                     @nspace="default"
                                     @partition="-" />`);

    await click(`${modalSelector} ${modalCancelButtonSelector}`);

    assert.ok(hideModal.called, 'hide method is called when cancel button is clicked');
  });

  test('it shows an alert when policy was not loaded and it is not possible to generate a token', async function (assert) {
    // creating a fake service that will return an empty policy
    const fakeService = class extends Service {
      close = close;
      open() {
        source.getCurrentEvent = function () {
          return {};
        };
        return source;
      }
    };
    this.owner.register('service:data-source/fake-service', fakeService);

    await render(hbs`<LinkToHcpModal @dc="dc-1"
                                     @nspace="default"
                                     @partition="-" />`);

    assert.dom(modalSelector).exists({ count: 1 });
    assert.dom(`${modalSelector} ${modalNoACLsAlertSelector}`).doesNotExist();
    // select read-only
    await click(`${modalSelector} ${modalOptionReadOnlySelector}`);

    // when read-only selected and no policy, it doesn't show the generate token button
    assert.dom(`${modalSelector} ${modalGenerateTokenButtonSelector}`).doesNotExist();
    // Missed policy alert is visible
    assert.dom(`${modalSelector} ${modalGenerateTokenMissedPolicyAlertSelector}`).isVisible();
  });

  test('it shows an error wher read-only selected and acls are disabled', async function (assert) {
    this.owner.register(
      'service:abilities',
      class Stub extends Service {
        can(permission) {
          if (permission === 'read acls') {
            return false;
          }
        }
      }
    );

    await render(hbs`<LinkToHcpModal @dc="dc-1"
                                     @nspace="default"
                                     @partition="-" />`);

    assert.dom(modalSelector).exists({ count: 1 });
    assert.dom(`${modalSelector} ${modalNoACLsAlertSelector}`).isVisible();
    // select read-only
    await click(`${modalSelector} ${modalOptionReadOnlySelector}`);

    // when read-only selected and no policy, it doesn't show the generate token button
    assert.dom(`${modalSelector} ${modalGenerateTokenButtonSelector}`).doesNotExist();
    // No acls enabled error is presented
    assert.dom(`${modalSelector} ${modalOptionReadOnlyErrorSelector}`).isVisible();
  });
});
