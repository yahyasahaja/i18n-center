'use client'

import { useRef, useState } from 'react'
import Editor from '@monaco-editor/react'

interface CodeEditorProps {
  value: string
  onChange: (value: string | undefined) => void
  language?: string
  height?: string
  readOnly?: boolean
}

export const CodeEditor: React.FC<CodeEditorProps> = ({
  value,
  onChange,
  language = 'json',
  height = '500px',
  readOnly = false,
}) => {
  const editorRef = useRef<any>(null)
  const [isEditorReady, setIsEditorReady] = useState(false)

  const handleEditorDidMount = (editor: any) => {
    editorRef.current = editor
    setIsEditorReady(true)
    // Configure editor options
    editor.updateOptions({
      minimap: { enabled: false },
      fontSize: 14,
      lineNumbers: 'on',
      wordWrap: 'on',
      formatOnPaste: true,
      formatOnType: true,
      tabSize: 2,
    })
  }

  const loadingComponent = (
    <div className="flex items-center justify-center" style={{ height }}>
      <div className="text-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-600 mx-auto"></div>
        <p className="mt-2 text-sm text-gray-600">Loading editor...</p>
      </div>
    </div>
  )

  return (
    <div className="border border-gray-300 rounded-md overflow-hidden">
      <Editor
        height={height}
        language={language}
        value={value}
        onChange={onChange}
        onMount={handleEditorDidMount}
        theme="vs-light"
        loading={loadingComponent}
        options={{
          readOnly,
          minimap: { enabled: false },
          fontSize: 14,
          lineNumbers: 'on',
          wordWrap: 'on',
          formatOnPaste: true,
          formatOnType: true,
          tabSize: 2,
          automaticLayout: true,
          scrollBeyondLastLine: false,
        }}
      />
    </div>
  )
}

