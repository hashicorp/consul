import { ReactNode } from 'react'
import classNames from 'classnames'
import s from './style.module.css'

interface SideBySideProps {
  left: ReactNode
  right: ReactNode
}

export default function SideBySide({ left, right }: SideBySideProps) {
  return (
    <div className={s.sideBySide}>
      <div className={classNames(s.sideWrapper, s.leftSide)}>
        <div className={s.side}>{left}</div>
      </div>
      <div className={classNames(s.sideWrapper, s.rightSide)}>
        <div className={s.side}>{right}</div>
      </div>
    </div>
  )
}
