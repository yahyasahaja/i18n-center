'use client'

import React from 'react'
import ReactDiffViewer from 'react-diff-viewer-continued'
import { Card } from './ui/Card'

interface DiffViewProps {
  oldValue: string
  newValue: string
  title?: string
}

export const DiffView: React.FC<DiffViewProps> = ({
  oldValue,
  newValue,
  title = 'Changes',
}) => {
  return (
    <Card title={title}>
      <div className="overflow-auto">
        <ReactDiffViewer
          oldValue={oldValue}
          newValue={newValue}
          splitView={true}
          leftTitle="Before (Version 1)"
          rightTitle="After (Version 2)"
          showDiffOnly={false}
          useDarkTheme={false}
        />
      </div>
    </Card>
  )
}

