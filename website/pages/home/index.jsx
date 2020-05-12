import Hero from '@hashicorp/react-hero'
import BeforeAfterDiagram from '../../components/before-after'
import SectionHeader from '@hashicorp/react-section-header'
import consulEnterpriseLogo from '../../public/img/consul-connect/logos/consul-enterprise-logo.svg?include'
import consulLogo from '../../public/img/consul-connect/logos/consul-logo.svg?include'

export default function HomePage() {
  return (
    <>
      <div className="consul-connect">
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
                },
                {
                  title: 'Get Started',
                  url:
                    'https://learn.hashicorp.com/consul/getting-started/install',
                  external: false,
                  theme: '',
                },
              ],
              helpText:
                '<a href="https://demo.consul.io">View demo of web UI</a>',
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
          className="g-section-block layout-vertical theme-white-background-black-text bg-light large-padding"
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
          className="g-section-block layout-vertical theme-white-background-black-text large-padding"
        >
          <div className="g-grid-container">
            <SectionHeader
              headline="Service-based networking for dynamic infrastructure"
              description="The shift from static infrastructure to dynamic infrastructure changes the approach to networking from host-based to service-based. Connectivity moves from the use of static IPs to dynamic service discovery, and security moves from static firewalls to service identity."
            />
            <BeforeAfterDiagram
              beforeHeading="Static"
              beforeSubTitle="Host-based networking"
              beforeImage="/img/consul-connect/svgs/static.svg"
              afterHeading="Dynamic"
              afterSubTitle="Service-based networking"
              afterImage="/img/consul-connect/svgs/dynamic.svg"
            />
          </div>
        </section>
        <section className="g-section bg-light border-top small-padding">
          <div className="g-container">
            <div className="g-text-asset">
              <div>
                <div>
                  <h3 className="g-type-display-3">Extend and Integrate</h3>
                  <p className="g-type-body">
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
                    srcSet="
              /img/consul-connect/grid_2/grid_2_300.webp 300w,
              /img/consul-connect/grid_2/grid_2_704.webp 704w,
              /img/consul-connect/grid_2/grid_2_1256.webp 1256w"
                  />
                  <source
                    type="image/png"
                    srcSet="
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
          className="g-section-block layout-vertical theme-light-gray large-padding"
        >
          <div className="g-container">
            <SectionHeader headline="Companies that trust Consul" />
            <div className="g-logo-grid">
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
        <section className="home-cta-section">
          <div>
            <div>
              <div
                dangerouslySetInnerHTML={{
                  __html: consulLogo,
                }}
              />
              <p className="g-type-body">
                Consul Open Source addresses the technical complexity of
                connecting services across distributed infrastructure.
              </p>
              <div>
                <a href="/downloads.html" className="button white download">
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="20"
                    height="22"
                    viewBox="0 0 20 22"
                  >
                    <path d="M9.292 15.706a1 1 0 0 0 1.416 0l3.999-3.999a1 1 0 1 0-1.414-1.414L11 12.586V1a1 1 0 1 0-2 0v11.586l-2.293-2.293a1 1 0 1 0-1.414 1.414l3.999 3.999zM20 16v3c0 1.654-1.346 3-3 3H3c-1.654 0-3-1.346-3-3v-3a1 1 0 1 1 2 0v3c0 .551.448 1 1 1h14c.552 0 1-.449 1-1v-3a1 1 0 1 1 2 0z" />
                  </svg>
                  Download
                </a>
              </div>
            </div>
          </div>
          <div>
            <div>
              <div
                dangerouslySetInnerHTML={{
                  __html: consulEnterpriseLogo,
                }}
              />
              <p className="g-type-body">
                Consul Enterprise addresses the organizational complexity of
                large user bases and compliance requirements with collaboration
                and governance features.
              </p>
              <div>
                <a
                  href="https://www.hashicorp.com/products/consul"
                  className="button secondary white"
                >
                  Learn More
                </a>
              </div>
            </div>
          </div>
        </section>
      </div>
    </>
  )
}
