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
          <div key={type} className={s.card}>
            <img src={image.src} alt={image.alt} />
            <span className={s.description}>{description}</span>
            <a href={cta.url}>{cta.text}</a>
          </div>
        ))}
      </div>
    </>
  )
}
