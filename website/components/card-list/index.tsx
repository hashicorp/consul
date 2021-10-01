import s from './style.module.css'

interface Card {
  image: {
    src: string
    alt: string
  }
  description: string
  url: string
  eyebrow: string
}

interface CardListProps {
  title: string
  cards: Card[]
  className?: string
}

export default function CardList({ title, cards, className }: CardListProps) {
  return (
    <div className={className}>
      <h3 className={s.title}>{title}</h3>
      <div className={s.cardsWrapper}>
        {cards.map(({ image, description, url, eyebrow }) => (
          <a
            href={url}
            key={eyebrow}
            className={s.card}
            target="_blank"
            rel="noreferrer"
          >
            <div className={s.imageContainer}>
              <img src={image.src} alt={image.alt} />
            </div>
            <div className={s.cardContent}>
              <div className={s.preHeader}>
                <span>{eyebrow}</span>
                <img
                  alt="external-link"
                  src={require('./images/external-link-icon.svg')}
                />
              </div>
              <span className={s.description}>{description}</span>
            </div>
          </a>
        ))}
      </div>
    </div>
  )
}
