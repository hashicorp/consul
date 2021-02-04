import s from './style.module.css'

function Cards({ children }) {
  return <div className={s.cards}>{children}</div>
}

export default Cards
