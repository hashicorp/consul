/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { click, render } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import Service from '@ember/service';
import sinon from 'sinon';
import { ACCESS_LEVEL } from 'consul-ui/components/link-to-hcp-modal';

const modalSelector = '[data-test-link-to-hcp-modal]';
const modalOptionReadOnlySelector = '#accessMode-readonly';
const modalGenerateTokenCardSelector = '[data-test-link-to-hcp-modal-generate-token-card]';
const modalGenerateTokenCardValueSelector =
  '[data-test-link-to-hcp-modal-generate-token-card-value]';
const modalGenerateTokenCardCopyButtonSelector =
  '[data-test-link-to-hcp-modal-generate-token-card-copy-button]';
const modalGenerateTokenButtonSelector = '[data-test-link-to-hcp-modal-generate-token-button]';
const modalNextButtonSelector = '[data-test-link-to-hcp-modal-next-button]';
const modalCancelButtonSelector = '[data-test-link-to-hcp-modal-cancel-button]';
const resourceId =
  'organization/b4432207-bb9c-438e-a160-b98923efa979/project/4b09958c-fa91-43ab-8029-eb28d8cee9d4/hashicorp.consul.global-network-manager.cluster/test-from-api';

module('Integration | Component | link-to-hcp-modal', function (hooks) {
  let originalClipboardWriteText;
  let hideModal = sinon.stub();

  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    this.owner.register(
      'service:abilities',
      class Stub extends Service {
        can(permission) {
          if (permission === 'create tokens') {
            return true;
          }
        }
      }
    );
    this.owner.register(
      'service:hcp-link-modal',
      class Stub extends Service {
        resourceId = resourceId;
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
    this.globalReadonlyPolicy = {
      data: {
        ID: '00000000-0000-0000-0000-000000000002',
        Name: 'global-readonly',
      },
    };

    await render(hbs`<LinkToHcpModal @policy={{this.globalReadonlyPolicy}} 
                            @dc='dc' 
                            @nspace='ns'
                            @partition='part' />`);

    assert.dom(modalSelector).exists({ count: 1 });
    // select read-only
    await click(`${modalSelector} ${modalOptionReadOnlySelector}`);

    // when read-only selected, it shows the generate token button
    assert.dom(`${modalSelector} ${modalGenerateTokenButtonSelector}`).isVisible();
  });

  test('it updates next link on option selected', async function (assert) {
    this.globalReadonlyPolicy = {
      data: {
        ID: '00000000-0000-0000-0000-000000000002',
        Name: 'global-readonly',
      },
    };

    await render(hbs`<LinkToHcpModal @policy={{this.globalReadonlyPolicy}} 
                            @dc='dc' 
                            @nspace='ns'
                            @partition='part' />`);

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
    this.globalReadonlyPolicy = {
      data: {
        ID: '00000000-0000-0000-0000-000000000002',
        Name: 'global-readonly',
      },
    };

    await render(hbs`<LinkToHcpModal @policy={{this.globalReadonlyPolicy}} 
                            @dc='dc' 
                            @nspace='ns'
                            @partition='part' />`);
    // select read-only
    await click(`${modalSelector} ${modalOptionReadOnlySelector}`);
    assert
      .dom(`${modalSelector} ${modalGenerateTokenButtonSelector}`)
      .hasText('Generate a read-only ACL token');

    await click(`${modalSelector} ${modalGenerateTokenButtonSelector}`);

    assert.dom(`${modalSelector} ${modalGenerateTokenCardSelector}`).isVisible();
    assert.dom(`${modalSelector} ${modalGenerateTokenCardValueSelector}`).exists();
    const tokenValue = this.element.querySelector(
      `${modalSelector} ${modalGenerateTokenCardValueSelector}`
    ).textContent;
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
    this.globalReadonlyPolicy = {
      data: {
        ID: '00000000-0000-0000-0000-000000000002',
        Name: 'global-readonly',
      },
    };

    await render(hbs`<LinkToHcpModal @policy={{this.globalReadonlyPolicy}} 
                            @dc='dc' 
                            @nspace='ns'
                            @partition='part' />`);

    await click(`${modalSelector} ${modalCancelButtonSelector}`);

    assert.ok(hideModal.called, 'hide method is called when cancel button is clicked');
  });
});
