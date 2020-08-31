import VERSION from '../../data/version.js'
import ProductDownloader from '@hashicorp/react-product-downloader'
import Head from 'next/head'
import HashiHead from '@hashicorp/react-head'

export default function DownloadsPage({ releaseData }) {
  return (
    <div id="p-downloads" className="g-container">
      <HashiHead is={Head} title="Downloads | Consul by HashiCorp" />
      <ProductDownloader
        product="Consul"
        version={VERSION}
        releaseData={releaseData}
      >
        <p>
          <a href="/downloads_tools">&raquo; Download Consul Tools</a>
        </p>
        <div className="release-candidate">
          <p>Note for ARM users:</p>

          <ul>
            <li>Use Armelv5 for all 32-bit armel systems</li>
            <li>Use Armhfv6 for all armhf systems with v6+ architecture</li>
            <li>Use Arm64 for all v8 64-bit architectures</li>
          </ul>

          <p>
            The following commands can help determine the right version for your
            system:
          </p>

          <code>$ uname -m</code>
          <br />
          <code>
            $ readelf -a /proc/self/exe | grep -q -c Tag_ABI_VFP_args && echo
            &quot;armhf&quot; || echo &quot;armel&quot;
          </code>
        </div>
      </ProductDownloader>
    </div>
  )
}

export async function getStaticProps() {
  return fetch(`https://releases.hashicorp.com/consul/${VERSION}/index.json`)
    .then((res) => res.json())
    .then((releaseData) => ({ props: { releaseData } }))
    .catch(() => {
      throw new Error(
        `--------------------------------------------------------
        Unable to resolve version ${VERSION} on releases.hashicorp.com from link
        <https://releases.hashicorp.com/consul/${VERSION}/index.json>. Usually this
        means that the specified version has not yet been released. The downloads page
        version can only be updated after the new version has been released, to ensure
        that it works for all users.
        ----------------------------------------------------------`
      )
    })
}
