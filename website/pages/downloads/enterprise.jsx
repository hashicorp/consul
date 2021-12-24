import VERSION from 'data/version'
import { productSlug } from 'data/metadata'
import ProductDownloadsPage from '@hashicorp/react-product-downloads-page'
import { generateStaticProps } from '@hashicorp/react-product-downloads-page/server'
import Head from 'next/head'
import s from './style.module.css'

export default function DownloadsPage(staticProps) {
  return (
    <>
      <Head>
        {/* Legal has requested that we not make this page public but not search engine indexable */}
        <meta name="robots" content="noindex, nofollow" />
      </Head>
      <ProductDownloadsPage
        enterpriseMode={true}
        getStartedDescription="Follow step-by-step tutorials on the essentials of Consul."
        getStartedLinks={[
          {
            label: 'CLI Quickstart',
            href:
              'https://learn.hashicorp.com/collections/consul/getting-started',
          },
          {
            label: 'HCP Consul',
            href:
              'https://learn.hashicorp.com/collections/consul/cloud-get-started',
          },
          {
            label: 'HCS on Azure',
            href: 'https://learn.hashicorp.com/collections/consul/hcs-azure',
          },
          {
            label: 'Kubernetes Quickstart',
            href:
              'https: //learn.hashicorp.com/collections/consul/gs-consul-service-mesh',
          },
          {
            label: 'View all Consul tutorials',
            href: 'https://learn.hashicorp.com/consul',
          },
        ]}
        logo={
          <img
            className={s.logo}
            alt="Consul"
            src={require('@hashicorp/mktg-logos/product/consul/primary/color.svg')}
          />
        }
        tutorialLink={{
          href: 'https://learn.hashicorp.com/consul',
          label: 'View Tutorials at HashiCorp Learn',
        }}
        merchandisingSlot={
          <>
            <p>
              <a href="/docs/download-tools">&raquo; Download Consul Tools</a>
            </p>
            <div className={s.releaseCandidate}>
              <p>Note for ARM users:</p>

              <ul>
                <li>Use Armelv5 for all 32-bit armel systems</li>
                <li>Use Armhfv6 for all armhf systems with v6+ architecture</li>
                <li>Use Arm64 for all v8 64-bit architectures</li>
              </ul>

              <p>
                The following commands can help determine the right version for
                your system:
              </p>

              <code>$ uname -m</code>
              <br />
              <code>
                $ readelf -a /proc/self/exe | grep -q -c Tag_ABI_VFP_args &&
                echo &quot;armhf&quot; || echo &quot;armel&quot;
              </code>
            </div>
            <div className={s.releaseCandidate}>
              <p>
                A beta for Consul v1.10.0 is available! The release can be{' '}
                <a href="https://releases.hashicorp.com/consul/1.10.0-beta3/">
                  downloaded here
                </a>
              </p>
            </div>
          </>
        }
        {...staticProps}
      />
    </>
  )
}

export async function getStaticProps() {
  return generateStaticProps({
    product: productSlug,
    latestVersion: VERSION,
  })
}
