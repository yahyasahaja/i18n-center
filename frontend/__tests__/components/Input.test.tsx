import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Input } from '@/components/ui/Input'

describe('Input', () => {
  it('renders an input element', () => {
    render(<Input />)
    expect(screen.getByRole('textbox')).toBeInTheDocument()
  })

  it('renders label when provided', () => {
    render(<Input label="Username" />)
    expect(screen.getByText('Username')).toBeInTheDocument()
  })

  it('does not render label element when label is not provided', () => {
    render(<Input />)
    expect(screen.queryByRole('label')).not.toBeInTheDocument()
  })

  it('shows error message when error prop is provided', () => {
    render(<Input error="This field is required" />)
    expect(screen.getByText('This field is required')).toBeInTheDocument()
  })

  it('shows helperText when provided and no error', () => {
    render(<Input helperText="Enter your email" />)
    expect(screen.getByText('Enter your email')).toBeInTheDocument()
  })

  it('does not show helperText when error is also provided', () => {
    render(<Input error="Required" helperText="Enter email" />)
    expect(screen.queryByText('Enter email')).not.toBeInTheDocument()
    expect(screen.getByText('Required')).toBeInTheDocument()
  })

  it('accepts user input', async () => {
    const user = userEvent.setup()
    render(<Input />)
    const input = screen.getByRole('textbox')
    await user.type(input, 'hello world')
    expect(input).toHaveValue('hello world')
  })

  it('forwards html attributes like placeholder', () => {
    render(<Input placeholder="Type here..." />)
    expect(screen.getByPlaceholderText('Type here...')).toBeInTheDocument()
  })

  it('applies error border class when error is provided', () => {
    render(<Input error="Error" />)
    expect(screen.getByRole('textbox').className).toContain('border-red-300')
  })

  it('applies normal border class when no error', () => {
    render(<Input />)
    expect(screen.getByRole('textbox').className).toContain('border-gray-300')
  })
})
