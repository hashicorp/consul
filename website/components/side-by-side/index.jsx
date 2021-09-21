import classNames from 'classNames'
import s from './style.module.css'

export default function SideBySide({ left, right }) {
  return (
    <div className={s.sideBySide}>
      <div className={classNames(s.sideContainer, s.leftSide)}>
        <div className={classNames(s.side, s.left)}>{left}</div>
      </div>
      <div className={classNames(s.sideContainer, s.rightSide)}>
        <div className={s.side}>{right}</div>
      </div>
    </div>
  )
}
