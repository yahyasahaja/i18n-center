import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Button } from '@/components/ui/Button'

describe('Button', () => {
  it('renders children text', () => {
    render(<Button>Click me</Button>)
    expect(screen.getByRole('button', { name: 'Click me' })).toBeInTheDocument()
  })

  it('calls onClick when clicked', async () => {
    const handleClick = jest.fn()
    const user = userEvent.setup()
    render(<Button onClick={handleClick}>Click</Button>)
    await user.click(screen.getByRole('button'))
    expect(handleClick).toHaveBeenCalledTimes(1)
  })

  it('is disabled when disabled prop is true', () => {
    render(<Button disabled>Disabled</Button>)
    expect(screen.getByRole('button')).toBeDisabled()
  })

  it('is disabled when isLoading is true', () => {
    render(<Button isLoading>Save</Button>)
    expect(screen.getByRole('button')).toBeDisabled()
  })

  it('shows Loading text when isLoading is true', () => {
    render(<Button isLoading>Save</Button>)
    expect(screen.getByRole('button')).toHaveTextContent('Loading...')
  })

  it('does not fire onClick when disabled', async () => {
    const handleClick = jest.fn()
    const user = userEvent.setup()
    render(<Button disabled onClick={handleClick}>Disabled</Button>)
    await user.click(screen.getByRole('button'))
    expect(handleClick).not.toHaveBeenCalled()
  })

  it('applies variant classes', () => {
    const { rerender } = render(<Button variant="danger">Danger</Button>)
    expect(screen.getByRole('button').className).toContain('bg-red-600')

    rerender(<Button variant="success">Success</Button>)
    expect(screen.getByRole('button').className).toContain('bg-green-600')

    rerender(<Button variant="outline">Outline</Button>)
    expect(screen.getByRole('button').className).toContain('border')
  })

  it('applies size classes', () => {
    const { rerender } = render(<Button size="sm">Small</Button>)
    expect(screen.getByRole('button').className).toContain('px-3')

    rerender(<Button size="lg">Large</Button>)
    expect(screen.getByRole('button').className).toContain('px-6')
  })

  it('forwards extra html attributes', () => {
    render(<Button type="submit" data-testid="submit-btn">Submit</Button>)
    const btn = screen.getByTestId('submit-btn')
    expect(btn).toHaveAttribute('type', 'submit')
  })
})
