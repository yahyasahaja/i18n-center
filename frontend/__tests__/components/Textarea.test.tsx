import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Textarea } from '@/components/ui/Textarea'

describe('Textarea', () => {
  it('renders a textarea element', () => {
    render(<Textarea />)
    expect(screen.getByRole('textbox')).toBeInTheDocument()
  })

  it('renders label when provided', () => {
    render(<Textarea label="Description" />)
    expect(screen.getByText('Description')).toBeInTheDocument()
  })

  it('does not render label when not provided', () => {
    const { container } = render(<Textarea />)
    expect(container.querySelector('label')).not.toBeInTheDocument()
  })

  it('shows error message when error is provided', () => {
    render(<Textarea error="This field is required" />)
    expect(screen.getByText('This field is required')).toBeInTheDocument()
  })

  it('shows helperText when provided and no error', () => {
    render(<Textarea helperText="Max 500 characters" />)
    expect(screen.getByText('Max 500 characters')).toBeInTheDocument()
  })

  it('does not show helperText when error is also provided', () => {
    render(<Textarea error="Required" helperText="Max 500 characters" />)
    expect(screen.queryByText('Max 500 characters')).not.toBeInTheDocument()
  })

  it('accepts user input', async () => {
    const user = userEvent.setup()
    render(<Textarea />)
    const textarea = screen.getByRole('textbox')
    await user.type(textarea, 'Hello world')
    expect(textarea).toHaveValue('Hello world')
  })

  it('applies error border class when error is provided', () => {
    render(<Textarea error="Error" />)
    expect(screen.getByRole('textbox').className).toContain('border-red-300')
  })

  it('applies normal border class when no error', () => {
    render(<Textarea />)
    expect(screen.getByRole('textbox').className).toContain('border-gray-300')
  })
})
