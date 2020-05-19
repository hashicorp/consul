import VERSION from '../../data/version.js'
import ProductDownloader from '@hashicorp/react-product-downloader'
import Head from 'next/head'
import HashiHead from '@hashicorp/react-head'

export default function DownloadsPage({ downloadData }) {
  return (
    <div id="p-downloads" className="g-container">
      <HashiHead is={Head} title="Downloads | Consul by HashiCorp" />
      <ProductDownloader
        product="Consul"
        version={VERSION}
        downloads={downloadData}
        prerelease={{
          type: 'beta', // the type of prerelease: beta, release candidate, etc.
          name: 'v1.8.0', // the name displayed in text on the website
          version: '1.8.0-beta1', // the actual version tag that was pushed to releases.hashicorp.com
        }}
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
    .then((r) => r.json())
    .then((r) => {
      // TODO: restructure product-downloader to run this logic internally
      return r.builds.reduce((acc, build) => {
        if (!acc[build.os]) acc[build.os] = {}
        acc[build.os][build.arch] = build.url
        return acc
      }, {})
    })
    .then((r) => ({ props: { downloadData: r } }))
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
