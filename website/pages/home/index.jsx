import marked from 'marked'
import Hero from '@hashicorp/react-hero'
// import TextAndContent from '@hashicorp/react-text-and-content'
// import Image from '@hashicorp/react-image'
// import CaseStudySlider from '@hashicorp/react-case-study-slider'
// import Button from '@hashicorp/react-button'
import BeforeAfterDiagram from '../../components/before-after'
import SectionHeader from '@hashicorp/react-section-header'
// import AlertBanner from '@hashicorp/react-alert-banner'

// import PageHeadTags from '../../../components/PageHeadTags'

export default function HomePage() {
  return (
    <>
      <div id="p-product-vault">
        {/* Hero */}
        <section id="hero">
          <Hero
            data={{
              centered: false,
              backgroundTheme: 'light',
              theme: 'consul-pink',
              smallTextTag: null,
              title: 'Secure Service Networking',
              description:
                'Consul is a service networking solution to connect and secure services across any runtime platform and public or private cloud',
              buttons: [
                {
                  title: 'Download',
                  url: '/downloads',
                  external: false,
                  theme: '',
                  gaPrefix: null,
                },
                {
                  title: 'Get Started',
                  url:
                    'https://learn.hashicorp.com/consul/getting-started/install',
                  external: false,
                  theme: '',
                  gaPrefix: null,
                },
              ],
              helpText: '<a href="View demo of web UI">View demo of web UI</a>',
              videos: [
                {
                  name: 'UI',
                  playbackRate: 2,
                  src: [
                    {
                      srcType: 'ogg',
                    },
                    {
                      srcType: 'webm',
                      url: '',
                    },
                    {
                      srcType: 'mp4',
                      url:
                        'https://consul-static-asssets.global.ssl.fastly.net/videos/v1/connect-video-ui.mp4',
                    },
                  ],
                },
                {
                  name: 'CLI',
                  src: [
                    {
                      srcType: 'mp4',
                      url:
                        'https://consul-static-asssets.global.ssl.fastly.net/videos/v1/connect-video-cli.mp4',
                    },
                  ],
                },
              ],
            }}
          />
        </section>
        {/* Use Cases */}
        <section
          id="vault-use-cases"
          className="g-section-block layout-vertical theme-white-background-black-text"
        >
          <div className="g-container">
            <SectionHeader
              headline="What can you do with Consul?"
              description="Consul is a service networking tool that allows you to discover services and secure network traffic."
            />

            <div className="g-use-cases">
              <div>
                <div>
                  <img
                    src="/img/consul-jtbd/kubernetes.png"
                    alt="Upgrade services"
                  />
                  <h3>Consul-Kubernetes Deployments</h3>
                  <p>
                    Use Consul service discovery and service mesh features with
                    Kubernetes.{' '}
                  </p>
                </div>
                <div>
                  <a
                    href="https://learn.hashicorp.com/consul/kubernetes/minikube?utm_source=consul.io&utm_medium=home-page&utm_content=jtbd&utm_term=jtbd-k8s"
                    className="button download"
                  >
                    Learn more
                  </a>
                </div>
              </div>
              <div>
                <div>
                  <img
                    src="/img/consul-jtbd/connect.png"
                    alt="Connect services"
                  />
                  <h3>Secure Service Communication</h3>
                  <p>
                    Secure and observe communication between your services
                    without modifying their code.
                  </p>
                </div>
                <div>
                  <a
                    href="https://learn.hashicorp.com/consul/getting-started/connect?utm_source=consul.io&utm_medium=home-page&utm_content=jtbd&utm_term=connect"
                    className="button download"
                  >
                    Learn more
                  </a>
                </div>
              </div>
              <div>
                <div>
                  <img
                    src="/img/consul-jtbd/load-balance.png"
                    alt="Load balance services"
                  />
                  <h3>Dynamic Load Balancing</h3>
                  <p>
                    Automate load balancer configuration with Consul and
                    HAProxy, Nginx, or F5.
                  </p>
                </div>
                <div>
                  <a
                    href="https://learn.hashicorp.com/consul/integrations/nginx-consul-template?utm_source=consul.io&utm_medium=home-page&utm_content=jtbd&utm_term=lb"
                    className="button download"
                  >
                    Learn more
                  </a>
                </div>
              </div>
            </div>
          </div>
        </section>
        {/* Static => Dynamic (Before & After) */}
        <section
          id="static-dynamic"
          className="g-section-block layout-vertical theme-white-background-black-text"
        >
          <div className="g-grid-container">
            <SectionHeader
              headline="Service-based networking for dynamic infrastructure"
              description="The shift from static infrastructure to dynamic infrastructure changes the approach to networking from host-based to service-based. Connectivity moves from the use of static IPs to dynamic service discovery, and security moves from static firewalls to service identity."
            />
            <BeforeAfterDiagram
              theme="nomad"
              beforeHeadline="Static"
              beforeContent="Host-based networking"
              beforeImage={{
                url: '/img/consul-connect/svgs/static.svg',
                alt: 'Static',
              }}
              afterHeadline="Dynamic"
              afterContent="Service-based networking"
              afterImage={{
                url: '/img/consul-connect/svgs/dynamic.svg',
                alt: 'Dynamic',
              }}
            />
          </div>
        </section>
        <section class="g-section bg-light border-top">
          <div class="g-container">
            <div class="g-text-asset">
              <div>
                <div>
                  <h3 class="g-type-display-3">Extend and Integrate</h3>
                  <p class="g-type-body">
                    Provision clusters on any infrastructure, connect to
                    services over TLS via proxy integrations, and Serve TLS
                    certificates with pluggable Certificate Authorities.
                  </p>
                </div>
              </div>
              <div>
                <picture>
                  <source
                    type="image/webp"
                    srcset="
              /img/consul-connect/grid_2/grid_2_300.webp 300w,
              /img/consul-connect/grid_2/grid_2_704.webp 704w,
              /img/consul-connect/grid_2/grid_2_1256.webp 1256w"
                  />
                  <source
                    type="image/png"
                    srcset="
              /img/consul-connect/grid_2/grid_2_300.png 300w,
              /img/consul-connect/grid_2/grid_2_704.png 704w,
              /img/consul-connect/grid_2/grid_2_1256.png 1256w"
                  />
                  <img
                    src="/img/consul-connect/grid_2/grid_2_1256.png"
                    alt="Extend and Integrate"
                  />
                </picture>
              </div>
            </div>
          </div>
        </section>

        {/* Companies Using Consul Logos */}
        <section
          id="companies-using-consul"
          className="g-section-block layout-vertical theme-light-gray"
        >
          <div className="g-container">
            <SectionHeader headline="Companies that trust Consul" />
            <div class="g-logo-grid">
              <div>
                <img
                  src="/img/consul-connect/logos/logo_sap-ariba_space.svg"
                  alt="SAP Ariba"
                />
              </div>
              <div>
                <img
                  src="/img/consul-connect/logos/logo_citadel_space.svg"
                  alt="Citadel"
                />
              </div>
              <div>
                <img
                  src="/img/consul-connect/logos/logo_barclays_space.svg"
                  alt="Barclays"
                />
              </div>
              <div>
                <img
                  src="/img/consul-connect/logos/logo_itv_space.svg"
                  alt="itv"
                />
              </div>
              <div>
                <img
                  src="/img/consul-connect/logos/logo_spaceflight-industries_space.svg"
                  alt="Spaceflight Industries"
                />
              </div>
              <div>
                <img
                  src="/img/consul-connect/logos/logo_lotto-nz_space.svg"
                  alt="MyLotto"
                />
              </div>
            </div>
          </div>
        </section>
      </div>
    </>
  )
}
