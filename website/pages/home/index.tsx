import * as React from 'react'
import Head from 'next/head'
import Image from 'next/image'
import rivetQuery from '@hashicorp/nextjs-scripts/dato/client'
import homepageQuery from './query.graphql'
import { renderMetaTags } from 'react-datocms'
import Button from '@hashicorp/react-button'
import IoHomeHero from 'components/io-home-hero'
import IoHomeIntro from 'components/io-home-intro'
import IoHomeInPractice from 'components/io-home-in-practice'
import IoVideoCallout from 'components/io-video-callout'
import IoCardContainer from 'components/io-card-container'
import IoHomeCaseStudies from 'components/io-home-case-studies'
import IoHomeCallToAction from 'components/io-home-call-to-action'
import IoHomePreFooter from 'components/io-home-pre-footer'
import s from './style.module.css'

export default function Homepage({ data }): React.ReactElement {
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
    inPracticeCtaHeading,
    inPracticeCtaDescription,
    inPracticeCtaLink,
    inPracticeCtaImage,
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
  } = data
  const _introVideo = introVideo[0]
  const _introOfferingsCta = introOfferingsCta[0]

  return (
    <>
      <Head>{renderMetaTags(seo)}</Head>

      <IoHomeHero
        pattern="/img/home-hero-pattern.svg"
        brand="consul"
        heading={heroHeading}
        description={heroDescription}
        ctas={heroCtas}
        cards={heroCards.map((card) => {
          return {
            ...card,
            cta: card.cta[0],
          }
        })}
      />

      <IoHomeIntro
        brand="consul"
        heading={introHeading}
        description={introDescription}
        offerings={{
          image: {
            src: introOfferingsImage.url,
            width: introOfferingsImage.width,
            height: introOfferingsImage.height,
            alt: introOfferingsImage.alt,
          },
          list: introOfferings,
          cta: {
            title: _introOfferingsCta.title,
            link: _introOfferingsCta.link,
          },
        }}
        video={{
          youtubeId: _introVideo.youtubeId,
          thumbnail: _introVideo.thumbnail.url,
          heading: _introVideo.heading,
          description: _introVideo.description,
          person: {
            name: _introVideo.personName,
            description: _introVideo.personDescription,
            avatar: _introVideo.personAvatar?.url,
          },
        }}
      />

      <IoHomeInPractice
        brand="consul"
        pattern="/img/practice-pattern.svg"
        heading={inPracticeHeading}
        description={inPracticeDescription}
        cards={inPracticeCards.map((card) => {
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
        cta={{
          heading: inPracticeCtaHeading,
          description: inPracticeCtaDescription,
          link: inPracticeCtaLink,
          image: inPracticeCtaImage,
        }}
      />

      <section className={s.useCases}>
        <div className={s.container}>
          <IoCardContainer
            heading={useCasesHeading}
            description={useCasesDescription}
            cardsPerRow={4}
            cards={useCasesCards.map((card) => {
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
            heading={tutorialsHeading}
            cardsPerRow={3}
            cards={tutorialCards.map((card) => {
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

      <IoHomeCaseStudies
        heading={caseStudiesHeading}
        description={caseStudiesDescription}
        primary={caseStudiesFeatured}
        secondary={caseStudiesLinks}
      />

      <IoHomeCallToAction
        brand="consul"
        heading={callToActionHeading}
        content={callToActionDescription}
        links={callToActionCtas}
      />

      <IoHomePreFooter
        brand="consul"
        heading={preFooterHeading}
        description={preFooterDescription}
        ctas={preFooterCtas}
      />
    </>
  )
}

export async function getStaticProps() {
  const { consulHomepage } = await rivetQuery({
    query: homepageQuery,
  })

  return {
    props: { data: consulHomepage },
    revalidate:
      process.env.HASHI_ENV === 'production'
        ? process.env.GLOBAL_REVALIDATE
        : 10,
  }
}
