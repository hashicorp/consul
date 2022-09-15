import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { render } from '@ember/test-helpers';
import Service from '@ember/service';

module('Integration | Component | consul bucket list', function (hooks) {
  setupRenderingTest(hooks);

  module('without nspace or partition feature on', function (hooks) {
    hooks.beforeEach(function () {
      this.owner.register(
        'service:abilities',
        class Stub extends Service {
          can(permission) {
            if (permission === 'use partitions') {
              return false;
            }
            if (permission === 'use nspaces') {
              return false;
            }

            return false;
          }
        }
      );
    });

    test('it displays a peer when the item passed has a peer name', async function (assert) {
      const PEER_NAME = 'Tomster';

      this.set('peerName', PEER_NAME);

      await render(hbs`
        <Consul::Bucket::List
          @item={{hash
            PeerName=this.peerName
            Namespace="default"
            Partition="default"
          }}
        />
      `);

      assert.dom('[data-test-bucket-item="peer"]').hasText(PEER_NAME, 'Peer name is displayed');
      assert.dom('[data-test-bucket-item="nspace"]').doesNotExist('namespace is not shown');
      assert.dom('[data-test-bucket-item="partition"]').doesNotExist('partition is not shown');
    });

    test('it does not display a bucket list when item has no peer name', async function (assert) {
      await render(hbs`
        <Consul::Bucket::List
          @item={{hash
            PeerName=this.peerName
          }}
        />
      `);

      assert.dom('[data-test-bucket-list]').doesNotExist('no bucket list displayed');
    });
  });

  module('with partition feature on', function (hooks) {
    hooks.beforeEach(function () {
      this.owner.register(
        'service:abilities',
        class Stub extends Service {
          can(permission) {
            if (permission === 'use partitions') {
              return true;
            }
            if (permission === 'use nspaces') {
              return true;
            }

            return false;
          }
        }
      );
    });

    test("it displays a peer and nspace and service and no partition when item.Partition and partition don't match", async function (assert) {
      const PEER_NAME = 'Tomster';
      const NAMESPACE_NAME = 'Mascot';
      const SERVICE_NAME = 'Ember.js';

      this.set('peerName', PEER_NAME);
      this.set('namespace', NAMESPACE_NAME);
      this.set('service', SERVICE_NAME);

      await render(hbs`
        <Consul::Bucket::List
          @item={{hash
            PeerName=this.peerName
            Namespace=this.namespace
            Service=this.service
            Partition="default"
          }}
          @partition="-"
          @nspace="-"
          @service="default"
        />
      `);

      assert.dom('[data-test-bucket-item="peer"]').hasText(PEER_NAME, 'Peer is displayed');
      assert
        .dom('[data-test-bucket-item="nspace"]')
        .hasText(NAMESPACE_NAME, 'namespace is displayed');
      assert.dom('[data-test-bucket-item="service"]').hasText(SERVICE_NAME, 'service is displayed');
      assert.dom('[data-test-bucket-item="partition"]').doesNotExist('partition is not displayed');
    });

    test("it displays partition and nspace and service when item.Partition and partition don't match and peer is not set", async function (assert) {
      const PARTITION_NAME = 'Ember.js';
      const NAMESPACE_NAME = 'Mascot';
      const SERVICE_NAME = 'Consul';

      this.set('partition', PARTITION_NAME);
      this.set('namespace', NAMESPACE_NAME);
      this.set('service', SERVICE_NAME);

      await render(hbs`
        <Consul::Bucket::List
          @item={{hash
            Namespace=this.namespace
            Service=this.service
            Partition=this.partition
          }}
          @partition="-"
          @nspace="-"
          @service="default"
        />
      `);

      assert.dom('[data-test-bucket-item="peer"]').doesNotExist('peer is not displayed');
      assert
        .dom('[data-test-bucket-item="nspace"]')
        .hasText(NAMESPACE_NAME, 'namespace is displayed');
      assert.dom('[data-test-bucket-item="service"]').hasText(SERVICE_NAME, 'service is displayed');
      assert
        .dom('[data-test-bucket-item="partition"]')
        .hasText(PARTITION_NAME, 'partition is displayed');
    });

    test('it displays nspace and peer and service when item.Partition and partition match and peer is set', async function (assert) {
      const PEER_NAME = 'Tomster';
      const PARTITION_NAME = 'Ember.js';
      const NAMESPACE_NAME = 'Mascot';
      const SERVICE_NAME = 'Ember.js';

      this.set('peerName', PEER_NAME);
      this.set('partition', PARTITION_NAME);
      this.set('namespace', NAMESPACE_NAME);
      this.set('service', SERVICE_NAME);

      await render(hbs`
        <Consul::Bucket::List
          @item={{hash
            PeerName=this.peerName
            Namespace=this.namespace
            Service=this.service
            Partition=this.partition
          }}
          @partition={{this.partition}}
          @nspace="-"
          @service="default"
        />
      `);

      assert.dom('[data-test-bucket-item="peer"]').hasText(PEER_NAME, 'peer is displayed');
      assert
        .dom('[data-test-bucket-item="nspace"]')
        .hasText(NAMESPACE_NAME, 'namespace is displayed');
      assert.dom('[data-test-bucket-item="service"]').hasText(SERVICE_NAME, 'service is displayed');
      assert.dom('[data-test-bucket-item="partition"]').doesNotExist('partition is not displayed');
    });
  });

  module('with nspace on but partition feature off', function (hooks) {
    hooks.beforeEach(function () {
      this.owner.register(
        'service:abilities',
        class Stub extends Service {
          can(permission) {
            if (permission === 'use partitions') {
              return false;
            }
            if (permission === 'use nspaces') {
              return true;
            }

            return false;
          }
        }
      );
    });

    test("it displays a peer and nspace and service when item.namespace and nspace don't match", async function (assert) {
      const PEER_NAME = 'Tomster';
      const NAMESPACE_NAME = 'Mascot';
      const SERVICE_NAME = 'Ember.js';

      this.set('peerName', PEER_NAME);
      this.set('namespace', NAMESPACE_NAME);
      this.set('service', SERVICE_NAME);

      await render(hbs`
        <Consul::Bucket::List
          @item={{hash
            PeerName=this.peerName
            Namespace=this.namespace
            Service=this.service
            Partition="default"
          }}
          @nspace="default"
          @service="default"
        />
      `);

      assert.dom('[data-test-bucket-item="peer"]').hasText(PEER_NAME, 'Peer is displayed');
      assert
        .dom('[data-test-bucket-item="nspace"]')
        .hasText(NAMESPACE_NAME, 'namespace is displayed');
      assert.dom('[data-test-bucket-item="service"]').hasText(SERVICE_NAME, 'service is displayed');
      assert.dom('[data-test-bucket-item="partition"]').doesNotExist('partition is not displayed');
    });

    test('it displays a peer and nspace when item.namespace and nspace match', async function (assert) {
      const PEER_NAME = 'Tomster';
      const NAMESPACE_NAME = 'Mascot';

      this.set('peerName', PEER_NAME);
      this.set('namespace', NAMESPACE_NAME);

      await render(hbs`
        <Consul::Bucket::List
          @item={{hash
            PeerName=this.peerName
            Namespace=this.namespace
            Partition="default"
          }}
          @nspace={{this.namespace}}
        />
      `);

      assert.dom('[data-test-bucket-item="peer"]').hasText(PEER_NAME, 'Peer is displayed');
      assert
        .dom('[data-test-bucket-item="nspace"]')
        .hasText(
          NAMESPACE_NAME,
          'namespace is displayed when peer is displayed and we are not on OSS (i.e. cannot use nspaces)'
        );
      assert.dom('[data-test-bucket-item="partition"]').doesNotExist('partition is not displayed');
    });
  });
});
