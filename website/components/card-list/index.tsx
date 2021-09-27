import s from './style.module.css'

interface Card {
  image: {
    src: string
    alt: string
  }
}

interface CardListProps {
  title: string
  cards: Card[]
}
export default function CardList({ title, cards }) {
  return <div className={s.root}></div>
}
