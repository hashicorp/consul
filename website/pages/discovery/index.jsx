import CallToAction from '@hashicorp/react-call-to-action'
import BeforeAfterDiagram from '../../components/before-after'
// import CaseStudySlider from '@hashicorp/react-case-study-slider'
// import TextSplitWithCode from '@hashicorp/react-text-split-with-code'

export default function ServiceDiscovery() {
  return (
    <>
      <div className="consul-connect">
        <CallToAction
          heading="Service discovery made easy"
          content="Service registry, integrated health checks, and DNS and HTTP interfaces enable any service to discover and be discovered by other services"
          brand="consul"
          links={[
            { text: 'Download', url: '/downloads' },
            {
              text: 'Explore Docs',
              url:
                'https://learn.hashicorp.com/consul/getting-started/services',
            },
          ]}
        />
        <section
          id="static-dynamic"
          className="g-section-block layout-vertical theme-white-background-black-text small-padding"
        >
          <div className="g-grid-container">
            <BeforeAfterDiagram
              beforeHeading="The Challenge"
              beforeSubTitle="Service load balancers aren't efficient in a dynamic world."
              beforeImage="/img/consul-connect/svgs/discovery-challenge.svg"
              beforeDescription="Load balancers are often used to front a service tier and provide a static IP. These load balancers add cost, increase latency, introduce single points of failure, and must be updated as services scale up/down."
              afterHeading="The Solution"
              afterSubTitle="Service discovery for dynamic infrastructure."
              afterImage="/img/consul-connect/svgs/discovery-solution.svg"
              afterDescription="Instead of load balancers, connectivity in dynamic infrastructure is best solved with service discovery. Service discovery uses a registry to keep a real-time list of services, their location, and their health. Services query the registry to discover the location of upstream services and then connect directly. This allows services to scale up/down and gracefully handle failure without a load balancer intermediary."
            />
          </div>
        </section>
        <section class="g-section border-top">
          <div class="g-container">
            <div class="intro">
              <h2 class="g-type-display-2">Features</h2>
            </div>
            <div class="g-text-asset large">
              <div>
                <div>
                  <h3 class="g-type-display-3">Service Registry</h3>
                  <p class="g-type-body">
                    Consul provides a registry of all the running nodes and
                    services, along with their current health status. This
                    allows operators to understand the environment, and
                    applications and automation tools to interact with dynamic
                    infrastructure using an HTTP API.
                  </p>
                  <p>
                    <a
                      class="learn-more g-type-buttons-and-standalone-links"
                      href="https://learn.hashicorp.com/consul/getting-started/services"
                    >
                      Learn more
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="6"
                        height="10"
                        viewBox="0 0 6 10"
                      >
                        <g
                          fill="none"
                          fill-rule="evenodd"
                          transform="translate(-6 -3)"
                        >
                          <mask id="a" fill="#fff">
                            <path d="M7.138 3.529a.666.666 0 1 0-.942.942l3.528 3.53-3.529 3.528a.666.666 0 1 0 .943.943l4-4a.666.666 0 0 0 0-.943l-4-4z" />
                          </mask>
                          <g fill="#1563FF" mask="url(#a)">
                            <path d="M0 0h16v16H0z" />
                          </g>
                        </g>
                      </svg>
                    </a>
                  </p>
                </div>
              </div>
              <div>
                <picture>
                  <source
                    type="image/webp"
                    srcset="
              /img/consul-connect/ui-health-checks/ui-health-checks_230.webp 230w,
              /img/consul-connect/ui-health-checks/ui-health-checks_690.webp 690w,
              /img/consul-connect/ui-health-checks/ui-health-checks_1290.webp 1290w"
                  />
                  <source
                    type="image/jpg"
                    srcset="
              /img/consul-connect/ui-health-checks/ui-health-checks_230.jpg 230w,
              /img/consul-connect/ui-health-checks/ui-health-checks_690.jpg 690w,
              /img/consul-connect/ui-health-checks/ui-health-checks_1290.jpg 1290w"
                  />
                  <img
                    src="/img/consul-connect/ui-health-checks/ui-health-checks_1290.jpg"
                    alt="Service Registry"
                  />
                </picture>
              </div>
            </div>
          </div>
        </section>
        {/* <TextSplitWithCode
          heading="Ttext"
          content="Test"
          code={`resource "digitalocean_droplet" "web" {
  name = "tf-web"n  size = "512mb"
  image = "centos-5-8-x32"
  region = "sfo1"
}
resource "dnsimple_record" "hello" {
  domain = "example.com"
  name = "test"
  value = "value"
  type = "A"
}`}
        /> */}

        {/* <section class='g-section border-top'>
    <div class='g-container'>
      <div class='g-text-asset reverse'>
        <div>
          <div>
            <h3 class="g-type-display-3">DNS Query Interface</h3>
            <p class="g-type-body">Consul enables service discovery using a built-in DNS server. This allows existing applications to easily integrate, as almost all applications support using DNS to resolve IP addresses. Using DNS instead of a static IP address allows services to scale up/down and route around failures easily.</p>
            <p>
              <a class="learn-more g-type-buttons-and-standalone-links" href="https://learn.hashicorp.com/consul/getting-started/services#querying-services">Learn more<svg xmlns="http://www.w3.org/2000/svg" width="6" height="10" viewBox="0 0 6 10"><g fill="none" fill-rule="evenodd" transform="translate(-6 -3)"><mask id="a" fill="#fff"><path d="M7.138 3.529a.666.666 0 1 0-.942.942l3.528 3.53-3.529 3.528a.666.666 0 1 0 .943.943l4-4a.666.666 0 0 0 0-.943l-4-4z"/></mask><g fill="#1563FF" mask="url(#a)"><path d="M0 0h16v16H0z"/></g></g></svg></a>
            </p>
          </div>
        </div>
        <div class='code-sample'>
          <div>
            <span></span>
            <div class='code'><code>
$ dig <code class='keyword'>web-frontend.service.consul.</code> ANY

; <<>> DiG 9.8.3-P1 <<>> web-frontend.service.consul. ANY
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 29981
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 2, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;web-frontend.service.consul. IN ANY

;; ANSWER SECTION:
web-frontend.service.consul. 0 IN A <code class='keyword'>10.0.3.83</code>
web-frontend.service.consul. 0 IN A <code class='keyword'>10.0.1.109</code>
</code>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section> */}

        {/* <section class='g-section border-top'>
    <div class='g-container'>
      <div class='g-text-asset'>
        <div>
          <div>
            <h3 class="g-type-display-3">HTTP API with Edge Triggers</h3>
            <p class="g-type-body">Consul provides an HTTP API to query the service registry for nodes, services, and health check information. The API also supports blocking queries, or long-polling for any changes. This allows automation tools to react to services being registered or health status changes to change configurations or traffic routing in real time.</p>
            <p>
              <a class="learn-more g-type-buttons-and-standalone-links" href="https://learn.hashicorp.com/consul/getting-started/services#http-api">Learn more<svg xmlns="http://www.w3.org/2000/svg" width="6" height="10" viewBox="0 0 6 10"><g fill="none" fill-rule="evenodd" transform="translate(-6 -3)"><mask id="a" fill="#fff"><path d="M7.138 3.529a.666.666 0 1 0-.942.942l3.528 3.53-3.529 3.528a.666.666 0 1 0 .943.943l4-4a.666.666 0 0 0 0-.943l-4-4z"/></mask><g fill="#1563FF" mask="url(#a)"><path d="M0 0h16v16H0z"/></g></g></svg></a>
            </p>
          </div>
        </div>
        <div class='code-sample'>
          <div>
            <span></span>
            <div class='code'>
              <code>$ curl <code class='keyword'>http://localhost:8500/v1/health/service/web?index=11&wait=30s</code>
{
  ...
  "Node": "10-0-1-109",
  "CheckID": "service:web",
  "Name": "Service 'web' check",
  "Status": <code class='keyword'>"critical"</code>,
  "ServiceID": "web",
  "ServiceName": "web",
  "CreateIndex": 10,
  "ModifyIndex": 20
  ...
}
</code>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section> */}
        {/*
  <section class='g-section border-top'>
    <div class='g-container'>
      <div class='g-text-asset reverse'>
        <div>
          <div>
            <h3 class="g-type-display-3">Multi Datacenter</h3>
            <p class="g-type-body">Consul supports multiple datacenters out of the box with no complicated configuration. Look up services in other datacenters or keep the request local. Advanced features like Prepared Queries enable automatic failover to other datacenters.</p>
            <p>
              <a class="learn-more g-type-buttons-and-standalone-links" href='https://learn.hashicorp.com/consul/security-networking/datacenters'>Learn more<svg xmlns="http://www.w3.org/2000/svg" width="6" height="10" viewBox="0 0 6 10"><g fill="none" fill-rule="evenodd" transform="translate(-6 -3)"><mask id="a" fill="#fff"><path d="M7.138 3.529a.666.666 0 1 0-.942.942l3.528 3.53-3.529 3.528a.666.666 0 1 0 .943.943l4-4a.666.666 0 0 0 0-.943l-4-4z"/></mask><g fill="#1563FF" mask="url(#a)"><path d="M0 0h16v16H0z"/></g></g></svg></a>
            </p>
          </div>
        </div>
        <div class='code-sample'>
          <div>
            <span></span>
            <div class='code'>
              <code>
$ curl http://localhost:8500/v1/catalog/datacenters
<code class='keyword'>[
  "dc1",
  "dc2"
]</code>
$ curl http://localhost:8500/v1/catalog/nodes?<code class='keyword'>dc=dc2</code>
[
    {
        "ID": "7081dcdf-fdc0-0432-f2e8-a357d36084e1",
        "Node": "10-0-1-109",
        "Address": "10.0.1.109",
        "Datacenter": "<code class='keyword'>dc2</code>",
        "TaggedAddresses": {
            "lan": "10.0.1.109",
            "wan": "10.0.1.109"
        },
        "CreateIndex": 112,
        "ModifyIndex": 125
    },
...</code>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section> */}

        <section class="g-section border-top">
          <div class="g-container">
            <div class="g-text-asset large">
              <div>
                <div>
                  <h3 class="g-type-display-3">Health Checks</h3>
                  <p class="g-type-body">
                    Pairing service discovery with health checking prevents
                    routing requests to unhealthy hosts and enables services to
                    easily provide circuit breakers.
                  </p>
                  <p>
                    <a
                      class="learn-more g-type-buttons-and-standalone-links"
                      href="https://learn.hashicorp.com/consul/getting-started/services"
                    >
                      Learn more
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="6"
                        height="10"
                        viewBox="0 0 6 10"
                      >
                        <g
                          fill="none"
                          fill-rule="evenodd"
                          transform="translate(-6 -3)"
                        >
                          <mask id="a" fill="#fff">
                            <path d="M7.138 3.529a.666.666 0 1 0-.942.942l3.528 3.53-3.529 3.528a.666.666 0 1 0 .943.943l4-4a.666.666 0 0 0 0-.943l-4-4z" />
                          </mask>
                          <g fill="#1563FF" mask="url(#a)">
                            <path d="M0 0h16v16H0z" />
                          </g>
                        </g>
                      </svg>
                    </a>
                  </p>
                </div>
              </div>
              <div>
                <picture>
                  <source
                    type="image/webp"
                    srcset="
              /img/consul-connect/ui-health-checks/ui-health-checks_230.webp 230w,
              /img/consul-connect/ui-health-checks/ui-health-checks_690.webp 690w,
              /img/consul-connect/ui-health-checks/ui-health-checks_1290.webp 1290w"
                  />
                  <source
                    type="image/jpg"
                    srcset="
              /img/consul-connect/ui-health-checks/ui-health-checks_230.jpg 230w,
              /img/consul-connect/ui-health-checks/ui-health-checks_690.jpg 690w,
              /img/consul-connect/ui-health-checks/ui-health-checks_1290.jpg 1290w"
                  />
                  <img
                    src="/img/consul-connect/ui-health-checks/ui-health-checks_1290.jpg"
                    alt="Health Checks"
                  />
                </picture>
              </div>
            </div>
          </div>
        </section>

        {/* <CaseStudySlider
          data={[
            {
              caseStudies: [
                {
                  company: {
                    monochromeLogo: {
                      url:
                        'https://www.datocms-assets.com/2885/1538067560-proofpoint-logo-reg-k.png',
                      alt: 'Logo dark',
                      format: 'png',
                    },
                    whiteLogo: {
                      url:
                        'https://www.datocms-assets.com/2885/1538067567-proofpoint-logo-reg-reversed.png',
                      alt: 'Logo white',
                      format: 'png',
                    },
                  },
                  headline: 'This is the first case study',
                  description:
                    'Lorem ipsum dolor sit amet, consectetur adipiscing elit. Phasellus ut arcu justo, et convallis lectus. Sed commodo massa eget risus feugiat suscipit. ',
                  caseStudyResource: {
                    slug: 'https://www.hashicorp.com',
                    image: {
                      url:
                        'https://www.datocms-assets.com/2885/1538142087-ye-endahl.jpg',
                      alt: 'Test image',
                      format: 'jpg',
                    },
                  },
                  buttonLabel: 'Custom Label',
                },
                {
                  company: {
                    monochromeLogo: {
                      url:
                        'https://www.datocms-assets.com/2885/1524097005-adobe-black-1.svg',
                      alt: 'Logo dark',
                      format: 'svg',
                    },
                    whiteLogo: {
                      url:
                        'https://www.datocms-assets.com/2885/1524097013-adobe-white-1.svg',
                      alt: 'Logo white',
                      format: 'svg',
                    },
                  },
                  headline: 'This is the second case study',
                  description:
                    'Lorem ipsum dolor sit amet, consectetur adipiscing elit. Phasellus ut arcu justo, et convallis lectus. Sed commodo massa eget risus feugiat suscipit. Nulla velit lectus, imperdiet cursus tempor at, ',
                  caseStudyResource: {
                    slug: 'https://www.hashicorp.com',
                    image: {
                      url:
                        'https://www.datocms-assets.com/2885/1538233406-wa-6h7-400x400.jpg',
                      alt: 'Test image',
                      format: 'jpg',
                    },
                  },
                },
                {
                  company: {
                    monochromeLogo: {
                      url:
                        'https://www.datocms-assets.com/2885/1535495419-black.png',
                      alt: 'Logo dark',
                      format: 'png',
                    },
                    whiteLogo: {
                      url:
                        'https://www.datocms-assets.com/2885/1535495424-white.png',
                      alt: 'Logo white',
                      format: 'png',
                    },
                  },
                  headline: 'This is the third case study',
                  description:
                    'Lorem ipsum dolor sit amet, consectetur adipiscing elit. Phasellus ut arcu justo, et convallis lectus. Sed commodo massa eget risus feugiat suscipit. Nulla velit lectus, ',
                  caseStudyResource: {
                    slug: 'https://www.hashicorp.com',
                    image: {
                      url:
                        'https://www.datocms-assets.com/2885/1535120026-36755049704aeaabe64ddk.jpg',
                      alt: 'Test image',
                      format: 'jpg',
                    },
                  },
                  caseStudyImage: {
                    url:
                      'https://www.datocms-assets.com/2885/1535120026-36755049704aeaabe64ddk.jpg',
                    alt: 'Case Study image override',
                    format: 'jpg',
                  },
                },
              ],
            },
          ]}
        /> */}
        <section class="g-section g-cta-section large-padding">
          <div>
            <h2 class="g-type-display-2">Ready to get started?</h2>
            <a href="/downloads.html" class="button download white">
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
            <a
              href="https://learn.hashicorp.com/consul/getting-started/services"
              class="button secondary white"
            >
              Explore docs
            </a>
          </div>
        </section>
      </div>
    </>
  )
}
