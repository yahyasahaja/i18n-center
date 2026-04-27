import React from 'react'
import { render, screen } from '@testing-library/react'
import { Card } from '@/components/ui/Card'

describe('Card', () => {
  it('renders children content', () => {
    render(<Card>Card body</Card>)
    expect(screen.getByText('Card body')).toBeInTheDocument()
  })

  it('renders title when provided', () => {
    render(<Card title="My Card">Content</Card>)
    expect(screen.getByText('My Card')).toBeInTheDocument()
  })

  it('does not render header section when neither title nor actions are provided', () => {
    const { container } = render(<Card>Content</Card>)
    expect(container.querySelector('.border-b')).not.toBeInTheDocument()
  })

  it('renders header section when title is provided', () => {
    const { container } = render(<Card title="Title">Content</Card>)
    expect(container.querySelector('.border-b')).toBeInTheDocument()
  })

  it('renders actions when provided', () => {
    render(<Card actions={<button>Action</button>}>Content</Card>)
    expect(screen.getByRole('button', { name: 'Action' })).toBeInTheDocument()
  })

  it('applies custom className', () => {
    const { container } = render(<Card className="custom-card">Content</Card>)
    expect(container.firstChild).toHaveClass('custom-card')
  })
})
