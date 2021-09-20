import s from './style.module.css'

export default function SideBySide({ left, right }) {
  return (
    <div className={s.sideBySide}>
      <div className={s.leftSide}>{left}</div>
      <div className={s.rightSide}>{right}</div>
    </div>
  )
}
