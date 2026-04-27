import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Modal } from '@/components/ui/Modal'

describe('Modal', () => {
  it('renders nothing when isOpen is false', () => {
    render(
      <Modal isOpen={false} onClose={jest.fn()} title="Test Modal">
        Content
      </Modal>
    )
    expect(screen.queryByText('Test Modal')).not.toBeInTheDocument()
  })

  it('renders title and children when isOpen is true', () => {
    render(
      <Modal isOpen={true} onClose={jest.fn()} title="My Modal">
        Modal body content
      </Modal>
    )
    expect(screen.getByText('My Modal')).toBeInTheDocument()
    expect(screen.getByText('Modal body content')).toBeInTheDocument()
  })

  it('calls onClose when the X button is clicked', async () => {
    const onClose = jest.fn()
    const user = userEvent.setup()
    render(
      <Modal isOpen={true} onClose={onClose} title="Test">
        Content
      </Modal>
    )
    // The X button is the close button
    const closeButtons = screen.getAllByRole('button')
    await user.click(closeButtons[0])
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when clicking the backdrop overlay', async () => {
    const onClose = jest.fn()
    const user = userEvent.setup()
    const { container } = render(
      <Modal isOpen={true} onClose={onClose} title="Test">
        Content
      </Modal>
    )
    // The backdrop is a fixed overlay div
    const backdrop = container.querySelector('.fixed.inset-0.bg-gray-500')
    await user.click(backdrop!)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('renders footer when provided', () => {
    render(
      <Modal
        isOpen={true}
        onClose={jest.fn()}
        title="Test"
        footer={<button>Save</button>}
      >
        Content
      </Modal>
    )
    expect(screen.getByRole('button', { name: 'Save' })).toBeInTheDocument()
  })

  it('does not render footer section when footer prop is absent', () => {
    const { container } = render(
      <Modal isOpen={true} onClose={jest.fn()} title="Test">
        Content
      </Modal>
    )
    expect(container.querySelector('.bg-gray-50')).not.toBeInTheDocument()
  })
})
