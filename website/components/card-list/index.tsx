import Link from 'next/link'
import Button from '@hashicorp/react-button'
import s from './style.module.css'

interface Card {
  image: {
    src: string
    alt: string
  }
  description: string
  cta: {
    url: string
    text: string
  }
  type: string
}

interface CardListProps {
  title: string
  cards: Card[]
}

export default function CardList({ title, cards }: CardListProps) {
  return (
    <>
      <h3 className={s.title}>{title}</h3>
      <div className={s.cardsWrapper}>
        {cards.map(({ image, description, cta, type }) => (
          <a href={cta.url} key={type} className={s.card}>
            <div className={s.imageContainer}>
              <img src={image.src} alt={image.alt} />
            </div>
            <div className={s.cardContent}>
              <div className={s.preHeader}>
                <span>{type}</span>
                <Button
                  title=""
                  url="#TODO"
                  linkType="outbound"
                  theme={{
                    variant: 'tertiary-neutral',
                  }}
                  className={s.ctaButton}
                />
              </div>
              <span className={s.description}>{description}</span>
              <span className={s.fauxLink}>{cta.text}</span>
            </div>
          </a>
        ))}
      </div>
    </>
  )
}
