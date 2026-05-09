'use client'

import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import Image from '@tiptap/extension-image'
import Link from '@tiptap/extension-link'
import TextAlign from '@tiptap/extension-text-align'
import { useCallback, useRef, useState, useEffect } from 'react'
import {
  Bold, Italic, List, ListOrdered,
  Link as LinkIcon, Image as ImageIcon,
  Undo, Redo, Code, Code2,
  AlignLeft, AlignCenter, AlignRight,
} from 'lucide-react'
import { cmsApi } from '@/services/api'
import toast from 'react-hot-toast'
import { html as beautifyHtml } from 'js-beautify'

function prettyHtml(raw: string): string {
  return beautifyHtml(raw, { indent_size: 2, wrap_line_length: 120 })
}

function minifyHtml(formatted: string): string {
  return formatted.replace(/>\s+</g, '><').trim()
}

// Width presets expressed as CSS percentages.
// Height is always `auto` so the aspect ratio is preserved — only the width can change.
const WIDTH_PRESETS = ['25%', '33%', '50%', '75%', '100%']

type ImageAlign = 'none' | 'left' | 'center' | 'right'

/**
 * Custom TipTap Image extension that persists `width` (CSS %) and `align` as inline styles.
 *
 * Why inline styles instead of classes?
 *   The HTML is stored as a raw string in the DB and rendered verbatim on any FE. CSS classes
 *   would depend on the consuming app's stylesheet, making rendering unpredictable. Inline styles
 *   are self-contained and render correctly anywhere.
 *
 * Alignment approach:
 *   - left/right  → CSS float (text wraps around the image)
 *   - center      → display:block + margin:auto (image on its own line, horizontally centred)
 *   - none        → default inline block (no float, no extra margin)
 *
 * PixelShift / srcset:
 *   Width is a CSS percentage — the pixel size is container-dependent and unknown at save time.
 *   srcset is therefore NOT stored in the HTML (it would bloat the saved content 5× per image
 *   and still be wrong for every viewport that differs from the editor's).
 *   Instead, consuming FE code should post-process PixelShift `src` URLs at render time to add
 *   responsive srcset. See i18ncenter-js README → "Rich text image srcset" for a utility snippet.
 *
 * Stored HTML example (50% width, centered):
 *   <img src="https://img.lapakgaming.com/?src=…"
 *        style="max-width:100%;height:auto;width:50%;display:block;margin-left:auto;margin-right:auto">
 */
const ResizableImage = Image.extend({
  addAttributes() {
    return {
      ...this.parent?.(),
      width: {
        default: null,
        parseHTML: (element) => {
          const el = element as HTMLElement
          return el.style.width || element.getAttribute('width') || null
        },
        // Suppress default attribute rendering — we merge everything in renderHTML below.
        renderHTML: () => ({}),
      },
      align: {
        default: 'none' as ImageAlign,
        parseHTML: (element) => {
          const el = element as HTMLElement
          if (el.style.float === 'left') return 'left'
          if (el.style.float === 'right') return 'right'
          if (el.style.display === 'block' && el.style.marginLeft === 'auto') return 'center'
          return 'none'
        },
        renderHTML: () => ({}),
      },
    }
  },

  renderHTML({ node, HTMLAttributes }) {
    // IMPORTANT: attribute-level `renderHTML: () => ({})` strips `width` and `align` from
    // HTMLAttributes (that's intentional — we don't want them as bare HTML attrs).
    // We must read their values from `node.attrs` directly instead.
    const width = node.attrs.width as string | null
    const align = node.attrs.align as ImageAlign

    const styles: string[] = ['max-width:100%', 'height:auto']

    if (width) styles.push(`width:${width}`)

    if (align === 'left') {
      styles.push('float:left', 'margin-right:1em', 'margin-bottom:0.5em')
    } else if (align === 'right') {
      styles.push('float:right', 'margin-left:1em', 'margin-bottom:0.5em')
    } else if (align === 'center') {
      // display:block + margin:auto is the standard way to center a replaced element.
      styles.push('display:block', 'margin-left:auto', 'margin-right:auto')
    }

    return ['img', { ...HTMLAttributes, style: styles.join(';') }]
  },
})

interface RichTextEditorProps {
  value: string
  onChange: (html: string) => void
  disabled?: boolean
  placeholder?: string
}

export function RichTextEditor({ value, onChange, disabled, placeholder }: RichTextEditorProps) {
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [htmlMode, setHtmlMode] = useState(false)
  const [rawHtml, setRawHtml] = useState(value || '')

  const editor = useEditor({
    immediatelyRender: false,
    extensions: [
      StarterKit,
      ResizableImage.configure({ inline: false, allowBase64: false }),
      Link.configure({ openOnClick: false, HTMLAttributes: { target: '_blank', rel: 'noopener noreferrer' } }),
      TextAlign.configure({ types: ['heading', 'paragraph'] }),
    ],
    content: value,
    editable: !disabled,
    onUpdate: ({ editor }) => {
      const html = editor.getHTML()
      setRawHtml(html)
      onChange(html)
    },
  })

  // Sync externally-changed value (e.g. after AI translate loads new content)
  useEffect(() => {
    if (!editor) return
    if (htmlMode) {
      setRawHtml(prettyHtml(value || ''))
    } else {
      const current = editor.getHTML()
      if ((value || '') !== current) {
        editor.commands.setContent(value || '')
        setRawHtml(value || '')
      }
    }
  }, [value]) // eslint-disable-line react-hooks/exhaustive-deps

  const switchToHtml = () => {
    if (!editor) return
    setRawHtml(prettyHtml(editor.getHTML()))
    setHtmlMode(true)
  }

  const switchToWysiwyg = () => {
    if (!editor) return
    const compact = minifyHtml(rawHtml)
    editor.commands.setContent(compact || '')
    onChange(compact)
    setHtmlMode(false)
  }

  const handleRawHtmlChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const v = e.target.value
    setRawHtml(v)
    onChange(v)
  }

  const handleImageUpload = useCallback(async (file: File) => {
    if (!editor) return
    try {
      const url = await cmsApi.uploadImage(file)
      editor.chain().focus().setImage({ src: url }).run()
    } catch {
      toast.error('Image upload failed')
    }
  }, [editor])

  const handleImageInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) handleImageUpload(file)
    e.target.value = ''
  }

  const setLink = useCallback(() => {
    if (!editor) return
    const prev = editor.getAttributes('link').href
    const url = window.prompt('URL', prev)
    if (url === null) return
    if (url === '') {
      editor.chain().focus().extendMarkRange('link').unsetLink().run()
      return
    }
    editor.chain().focus().extendMarkRange('link').setLink({ href: url }).run()
  }, [editor])

  const insertImageByUrl = useCallback(() => {
    if (!editor) return
    const url = window.prompt('Image URL')
    if (!url) return
    editor.chain().focus().setImage({ src: url }).run()
  }, [editor])

  if (!editor) return null

  const isImageSelected = editor.isActive('image')
  const imgAttrs = editor.getAttributes('image')
  const imgWidth: string = imgAttrs.width || '100%'
  const imgAlign: ImageAlign = imgAttrs.align || 'none'

  // No .focus() here — calling focus() can reset the NodeSelection (the selected image)
  // before updateAttributes runs, causing the update to silently do nothing.
  const setImgAttr = (attrs: { width?: string; align?: ImageAlign }) =>
    editor.chain().updateAttributes('image', attrs).run()

  const ToolButton = ({
    onClick, active, title, children,
  }: { onClick: () => void; active?: boolean; title: string; children: React.ReactNode }) => (
    <button
      type="button"
      onMouseDown={(e) => { e.preventDefault(); onClick() }}
      title={title}
      className={`p-1.5 rounded text-sm hover:bg-gray-100 transition-colors ${active ? 'bg-gray-200 text-gray-900' : 'text-gray-600'} ${disabled ? 'opacity-50 cursor-not-allowed' : ''}`}
      disabled={disabled}
    >
      {children}
    </button>
  )

  const isEmpty = !editor.getText().trim()

  return (
    <div className="border border-gray-300 rounded-md overflow-hidden flex flex-col min-h-[260px]">
      {/* Toolbar */}
      <div className="flex flex-wrap items-center gap-0.5 px-2 py-1 bg-gray-50 border-b border-gray-200 shrink-0">
        {!htmlMode && (
          <>
            {isImageSelected ? (
              /* Image-specific controls — replaces the normal toolbar while an image is selected */
              <>
                <span className="text-xs text-gray-400 px-1 select-none">Width:</span>
                {WIDTH_PRESETS.map((w) => (
                  <ToolButton
                    key={w}
                    onClick={() => setImgAttr({ width: w })}
                    active={imgWidth === w}
                    title={`Set image width to ${w} (height auto — ratio preserved)`}
                  >
                    <span className="text-xs font-medium leading-none">{w}</span>
                  </ToolButton>
                ))}
                <div className="w-px h-5 bg-gray-300 mx-1" />
                <span className="text-xs text-gray-400 px-1 select-none">Float:</span>
                <ToolButton
                  onClick={() => setImgAttr({ align: 'left' })}
                  active={imgAlign === 'left'}
                  title="Float image left (text wraps right)"
                >
                  <AlignLeft className="w-4 h-4" />
                </ToolButton>
                <ToolButton
                  onClick={() => setImgAttr({ align: 'center' })}
                  active={imgAlign === 'center'}
                  title="Center image (block, no float)"
                >
                  <AlignCenter className="w-4 h-4" />
                </ToolButton>
                <ToolButton
                  onClick={() => setImgAttr({ align: 'right' })}
                  active={imgAlign === 'right'}
                  title="Float image right (text wraps left)"
                >
                  <AlignRight className="w-4 h-4" />
                </ToolButton>
                <ToolButton
                  onClick={() => setImgAttr({ align: 'none' })}
                  active={imgAlign === 'none'}
                  title="No float (inline block)"
                >
                  <span className="text-xs text-gray-500 leading-none font-medium">—</span>
                </ToolButton>
                <div className="w-px h-5 bg-gray-300 mx-1" />
              </>
            ) : (
              /* Normal toolbar */
              <>
                {/* Headings */}
                <ToolButton
                  onClick={() => editor.chain().focus().toggleHeading({ level: 1 }).run()}
                  active={editor.isActive('heading', { level: 1 })}
                  title="Heading 1"
                >
                  <span className="text-xs font-bold leading-none">H1</span>
                </ToolButton>
                <ToolButton
                  onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
                  active={editor.isActive('heading', { level: 2 })}
                  title="Heading 2"
                >
                  <span className="text-xs font-bold leading-none">H2</span>
                </ToolButton>
                <ToolButton
                  onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()}
                  active={editor.isActive('heading', { level: 3 })}
                  title="Heading 3"
                >
                  <span className="text-xs font-bold leading-none">H3</span>
                </ToolButton>
                <div className="w-px h-5 bg-gray-300 mx-1" />

                {/* Text alignment (paragraphs / headings) */}
                <ToolButton
                  onClick={() => editor.chain().focus().setTextAlign('left').run()}
                  active={editor.isActive({ textAlign: 'left' })}
                  title="Align text left"
                >
                  <AlignLeft className="w-4 h-4" />
                </ToolButton>
                <ToolButton
                  onClick={() => editor.chain().focus().setTextAlign('center').run()}
                  active={editor.isActive({ textAlign: 'center' })}
                  title="Align text center"
                >
                  <AlignCenter className="w-4 h-4" />
                </ToolButton>
                <ToolButton
                  onClick={() => editor.chain().focus().setTextAlign('right').run()}
                  active={editor.isActive({ textAlign: 'right' })}
                  title="Align text right"
                >
                  <AlignRight className="w-4 h-4" />
                </ToolButton>
                <div className="w-px h-5 bg-gray-300 mx-1" />

                {/* Inline formatting */}
                <ToolButton onClick={() => editor.chain().focus().toggleBold().run()} active={editor.isActive('bold')} title="Bold">
                  <Bold className="w-4 h-4" />
                </ToolButton>
                <ToolButton onClick={() => editor.chain().focus().toggleItalic().run()} active={editor.isActive('italic')} title="Italic">
                  <Italic className="w-4 h-4" />
                </ToolButton>
                <ToolButton onClick={() => editor.chain().focus().toggleCode().run()} active={editor.isActive('code')} title="Inline code">
                  <Code className="w-4 h-4" />
                </ToolButton>
                <div className="w-px h-5 bg-gray-300 mx-1" />

                {/* Lists */}
                <ToolButton onClick={() => editor.chain().focus().toggleBulletList().run()} active={editor.isActive('bulletList')} title="Bullet list">
                  <List className="w-4 h-4" />
                </ToolButton>
                <ToolButton onClick={() => editor.chain().focus().toggleOrderedList().run()} active={editor.isActive('orderedList')} title="Numbered list">
                  <ListOrdered className="w-4 h-4" />
                </ToolButton>
                <div className="w-px h-5 bg-gray-300 mx-1" />

                {/* Link & image insertion */}
                <ToolButton onClick={setLink} active={editor.isActive('link')} title="Add link">
                  <LinkIcon className="w-4 h-4" />
                </ToolButton>
                <ToolButton onClick={() => fileInputRef.current?.click()} title="Upload image from file">
                  <ImageIcon className="w-4 h-4" />
                </ToolButton>
                <ToolButton onClick={insertImageByUrl} title="Insert image by URL">
                  <span className="text-xs font-medium leading-none">IMG URL</span>
                </ToolButton>
                <div className="w-px h-5 bg-gray-300 mx-1" />
              </>
            )}

            {/* History — always visible in WYSIWYG mode */}
            <ToolButton onClick={() => editor.chain().focus().undo().run()} title="Undo">
              <Undo className="w-4 h-4" />
            </ToolButton>
            <ToolButton onClick={() => editor.chain().focus().redo().run()} title="Redo">
              <Redo className="w-4 h-4" />
            </ToolButton>
            <div className="w-px h-5 bg-gray-300 mx-1" />
          </>
        )}

        {/* HTML source toggle — always visible */}
        <button
          type="button"
          onMouseDown={(e) => { e.preventDefault(); htmlMode ? switchToWysiwyg() : switchToHtml() }}
          title={htmlMode ? 'Switch to rich text editor' : 'Edit raw HTML (supports iframes, YouTube embeds, etc.)'}
          className={`p-1.5 rounded text-sm transition-colors ${htmlMode ? 'bg-blue-100 text-blue-600' : 'text-gray-600 hover:bg-gray-100'} ${disabled ? 'opacity-50 cursor-not-allowed' : ''}`}
          disabled={disabled}
        >
          <Code2 className="w-4 h-4" />
        </button>
      </div>

      {/* Editor body */}
      {htmlMode ? (
        <textarea
          className="w-full p-3 font-mono text-sm text-gray-900 bg-white min-h-[200px] focus:outline-none resize-y"
          value={rawHtml}
          onChange={handleRawHtmlChange}
          disabled={disabled}
          placeholder={`Enter HTML… (supports <h1>–<h6>, <iframe> for YouTube embeds, etc.)\n\nYouTube example:\n<iframe width="560" height="315" src="https://www.youtube.com/embed/VIDEO_ID" allowfullscreen></iframe>`}
          spellCheck={false}
        />
      ) : (
        <div
          className={`relative flex flex-col flex-1 cursor-text ${disabled ? 'bg-gray-50' : 'bg-white'}`}
          onClick={() => { if (!disabled) editor.chain().focus().run() }}
        >
          {isEmpty && placeholder && !disabled && (
            <div className="absolute top-0 left-0 px-3 py-3 text-sm text-gray-400 pointer-events-none select-none">
              {placeholder}
            </div>
          )}
          <EditorContent editor={editor} className="rich-text-editor" />
        </div>
      )}

      <input
        ref={fileInputRef}
        type="file"
        accept="image/jpeg,image/png,image/gif,image/webp"
        className="hidden"
        onChange={handleImageInputChange}
      />
    </div>
  )
}
