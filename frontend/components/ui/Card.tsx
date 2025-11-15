import React from 'react'
import { clsx } from 'clsx'

interface CardProps {
  children: React.ReactNode
  className?: string
  title?: string
  actions?: React.ReactNode
}

export const Card: React.FC<CardProps> = ({ children, className, title, actions }) => {
  return (
    <div className={clsx('bg-white shadow rounded-lg', className)}>
      {(title || actions) && (
        <div className="px-4 py-5 sm:px-6 border-b border-gray-200 flex items-center justify-between">
          {title && <h3 className="text-lg leading-6 font-medium text-gray-900">{title}</h3>}
          {actions && <div>{actions}</div>}
        </div>
      )}
      <div className="px-4 py-5 sm:p-6">{children}</div>
    </div>
  )
}

