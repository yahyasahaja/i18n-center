import React from 'react'
import { render, screen } from '@testing-library/react'
import { Badge } from '@/components/ui/Badge'

describe('Badge', () => {
  it('renders children text', () => {
    render(<Badge>Active</Badge>)
    expect(screen.getByText('Active')).toBeInTheDocument()
  })

  it('applies primary variant classes by default', () => {
    render(<Badge>Default</Badge>)
    expect(screen.getByText('Default').className).toContain('bg-primary-100')
  })

  it('applies danger variant classes', () => {
    render(<Badge variant="danger">Error</Badge>)
    expect(screen.getByText('Error').className).toContain('bg-red-100')
  })

  it('applies success variant classes', () => {
    render(<Badge variant="success">OK</Badge>)
    expect(screen.getByText('OK').className).toContain('bg-green-100')
  })

  it('applies warning variant classes', () => {
    render(<Badge variant="warning">Warn</Badge>)
    expect(screen.getByText('Warn').className).toContain('bg-yellow-100')
  })

  it('applies info variant classes', () => {
    render(<Badge variant="info">Info</Badge>)
    expect(screen.getByText('Info').className).toContain('bg-blue-100')
  })

  it('applies sm size classes', () => {
    render(<Badge size="sm">Small</Badge>)
    expect(screen.getByText('Small').className).toContain('text-xs')
  })

  it('applies md size classes by default', () => {
    render(<Badge>Default size</Badge>)
    expect(screen.getByText('Default size').className).toContain('text-sm')
  })

  it('applies custom className', () => {
    render(<Badge className="custom-class">Custom</Badge>)
    expect(screen.getByText('Custom').className).toContain('custom-class')
  })
})
