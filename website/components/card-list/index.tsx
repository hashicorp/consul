import s from './style.module.css'

interface Card {
  heading: string
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
        {cards.map(({ heading, description, url, eyebrow }) => (
          <a
            href={url}
            key={eyebrow}
            className={s.card}
            target="_blank"
            rel="noreferrer"
          >
            <div className={s.cardContent}>
              <span className={s.eyebrow}>{eyebrow}</span>
              <span className={s.heading}>{heading}</span>
              <p className={s.description}>{description}</p>
            </div>
            <img
              src={require('@hashicorp/mktg-logos/product/consul/logomark/color.svg')}
              alt="consul-icon"
              className={s.icon}
            />
          </a>
        ))}
      </div>
    </div>
  )
}
