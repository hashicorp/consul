import * as React from 'react'
import Head from 'next/head'
import Image from 'next/image'
import rivetQuery from '@hashicorp/nextjs-scripts/dato/client'
import homepageQuery from './query.graphql'
import { renderMetaTags } from 'react-datocms'
import Button from '@hashicorp/react-button'
import IoHomeHero from 'components/io-home-hero'
import IoVideoCallout from 'components/io-video-callout'
import IoCardContainer from 'components/io-card-container'
import IoHomeCaseStudies from 'components/io-home-case-studies'
import IoHomeCallToAction from 'components/io-home-call-to-action'
import IoHomePreFooter from 'components/io-home-pre-footer'
import s from './style.module.css'

export default function Homepage({ data }): React.ReactElement {
  const {
    seo,
    hero,
    intro,
    inPractice,
    useCases,
    caseStudies,
    tutorials,
    callToAction,
    preFooter,
  } = data

  return (
    <>
      <Head>{renderMetaTags(seo)}</Head>

      <IoHomeHero
        pattern="/img/home-hero-pattern.svg"
        brand="consul"
        {...hero}
      />

      <section className={s.intro}>
        <header className={s.introHeader}>
          <div className={s.container}>
            <div className={s.introHeaderInner}>
              <h2 className={s.introHeading}>{intro.heading}</h2>
              <p className={s.introDescription}>{intro.description}</p>
            </div>
          </div>
        </header>

        <div className={s.offerings}>
          {intro.offerings.image ? (
            <div className={s.offeringsMedia}>
              <Image
                src={intro.offerings.image.url}
                width={intro.offerings.image.width}
                height={intro.offerings.image.height}
                alt={intro.offerings.image.alt}
              />
            </div>
          ) : null}
          <div className={s.offeringsContent}>
            <ul className={s.offeringsList}>
              {intro.offerings.list.map((offering, index) => {
                return (
                  // Index is stable
                  // eslint-disable-next-line react/no-array-index-key
                  <li key={index}>
                    <h3 className={s.offeringsListHeading}>
                      {offering.heading}
                    </h3>
                    <p className={s.offeringsListDescription}>
                      {offering.description}
                    </p>
                  </li>
                )
              })}
            </ul>
            {intro.offerings.cta ? (
              <div className={s.offeringsCta}>
                <Button
                  title={intro.offerings.cta.title}
                  url={intro.offerings.cta.link}
                  theme={{
                    brand: 'neutral',
                  }}
                />
              </div>
            ) : null}
          </div>
        </div>

        {intro.video ? (
          <div className={s.container}>
            <IoVideoCallout
              youtubeId={intro.video.youtubeId}
              thumbnail={intro.video.thumbnail.url}
              heading={intro.video.heading}
              description={intro.video.description}
              person={{
                name: intro.video.personName,
                description: intro.video.personDescription,
                avatar: intro.video.personAvatar?.url,
              }}
            />
          </div>
        ) : null}
      </section>

      <section className={s.inPractice}>
        <div className={s.container}>
          <IoCardContainer
            theme="dark"
            heading={inPractice.heading}
            description={inPractice.description}
            cardsPerRow={3}
            cards={inPractice.cards.map((card) => {
              return {
                eyebrow: card.eyebrow,
                link: {
                  url: card.link,
                  type: 'inbound',
                },
                heading: card.heading,
                description: card.description,
                products: card.products,
              }
            })}
          />
        </div>
      </section>

      <section className={s.useCases}>
        <div className={s.container}>
          <IoCardContainer
            heading={useCases.heading}
            description={useCases.description}
            cardsPerRow={4}
            cards={useCases.cards.map((card) => {
              return {
                eyebrow: card.eyebrow,
                link: {
                  url: card.link,
                  type: 'inbound',
                },
                heading: card.heading,
                description: card.description,
                products: card.products,
              }
            })}
          />
        </div>
      </section>

      <section className={s.tutorials}>
        <div className={s.container}>
          <IoCardContainer
            heading={tutorials.heading}
            cardsPerRow={3}
            cards={tutorials.cards.map((card) => {
              return {
                eyebrow: card.eyebrow,
                link: {
                  url: card.link,
                  type: 'inbound',
                },
                heading: card.heading,
                description: card.description,
                products: card.products,
              }
            })}
          />
        </div>
      </section>

      <section className={s.caseStudies}>
        <div className={s.container}>
          <header className={s.caseStudiesHeader}>
            <h2 className={s.caseStudiesHeading}>{caseStudies.heading}</h2>
            <p className={s.caseStudiesDescription}>
              {caseStudies.description}
            </p>
          </header>

          <IoHomeCaseStudies
            primary={caseStudies.features}
            secondary={caseStudies.links}
          />
        </div>
      </section>

      <IoHomeCallToAction
        brand="consul"
        heading={callToAction.heading}
        content={callToAction.description}
        links={callToAction.links.map(({ text, url }, index) => {
          return {
            text,
            url,
            type: index === 1 ? 'inbound' : null,
          }
        })}
      />

      <IoHomePreFooter
        brand="consul"
        heading={preFooter.heading}
        description={preFooter.description}
        ctas={preFooter.ctas}
      />
    </>
  )
}

export async function getStaticProps() {
  const { consulHomepage } = await rivetQuery({
    query: homepageQuery,
  })

  const {
    seo,
    heroHeading,
    heroDescription,
    heroCtas,
    heroCards,
    introHeading,
    introDescription,
    introOfferingsImage,
    introOfferings,
    introOfferingsCta,
    introVideo,
    inPracticeHeading,
    inPracticeDescription,
    inPracticeCards,
    useCasesHeading,
    useCasesDescription,
    useCasesCards,
    tutorialsHeading,
    tutorialCards,
    caseStudiesHeading,
    caseStudiesDescription,
    caseStudiesFeatured,
    caseStudiesLinks,
    callToActionHeading,
    callToActionDescription,
    callToActionCtas,
    preFooterHeading,
    preFooterDescription,
    preFooterCtas,
  } = consulHomepage

  return {
    props: {
      data: {
        seo,
        hero: {
          heading: heroHeading,
          description: heroDescription,
          ctas: heroCtas,
          cards: heroCards.map((card) => {
            return {
              ...card,
              cta: card.cta[0],
            }
          }),
        },
        intro: {
          heading: introHeading,
          description: introDescription,
          offerings: {
            image: introOfferingsImage,
            list: introOfferings,
            cta: introOfferingsCta[0],
          },
          video: introVideo[0],
        },
        inPractice: {
          heading: inPracticeHeading,
          description: inPracticeDescription,
          cards: inPracticeCards,
        },
        useCases: {
          heading: useCasesHeading,
          description: useCasesDescription,
          cards: useCasesCards,
        },
        tutorials: {
          heading: tutorialsHeading,
          cards: tutorialCards,
        },
        caseStudies: {
          heading: caseStudiesHeading,
          description: caseStudiesDescription,
          features: caseStudiesFeatured,
          links: caseStudiesLinks,
        },
        callToAction: {
          heading: callToActionHeading,
          description: callToActionDescription,
          links: callToActionCtas,
        },
        preFooter: {
          heading: preFooterHeading,
          description: preFooterDescription,
          ctas: preFooterCtas,
        },
      },
    },
  }
}
