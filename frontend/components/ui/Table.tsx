import React from 'react'

interface TableProps {
  headers: string[]
  children: React.ReactNode
}

export const Table: React.FC<TableProps> = ({ headers, children }) => {
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full divide-y divide-gray-200">
        <thead className="bg-gray-50">
          <tr>
            {headers.map((header, index) => (
              <th
                key={index}
                className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
              >
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="bg-white divide-y divide-gray-200">{children}</tbody>
      </table>
    </div>
  )
}

interface TableRowProps {
  children: React.ReactNode
  // onClick receives the full MouseEvent so callers can inspect the click
  // target (e.g. ignore clicks that originated inside a nested button so the
  // row click doesn't fight per-row action buttons).
  onClick?: (e: React.MouseEvent<HTMLTableRowElement>) => void
  className?: string
}

export const TableRow: React.FC<TableRowProps> = ({ children, onClick, className }) => {
  const baseClass = onClick ? 'cursor-pointer hover:bg-gray-50' : ''
  return (
    <tr
      className={[baseClass, className].filter(Boolean).join(' ')}
      onClick={onClick}
    >
      {children}
    </tr>
  )
}

interface TableCellProps {
  children: React.ReactNode
  className?: string
}

export const TableCell: React.FC<TableCellProps> = ({ children, className }) => {
  return (
    <td className={`px-6 py-4 whitespace-nowrap text-sm text-gray-900 ${className || ''}`}>
      {children}
    </td>
  )
}

