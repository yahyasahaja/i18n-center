import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Select } from '@/components/ui/Select'

const options = [
  { value: 'draft', label: 'Draft' },
  { value: 'staging', label: 'Staging' },
  { value: 'production', label: 'Production' },
]

describe('Select', () => {
  it('renders a select element with options', () => {
    render(<Select options={options} />)
    expect(screen.getByRole('combobox')).toBeInTheDocument()
    expect(screen.getAllByRole('option')).toHaveLength(3)
  })

  it('renders label when provided', () => {
    render(<Select options={options} label="Environment" />)
    expect(screen.getByText('Environment')).toBeInTheDocument()
  })

  it('renders all option labels', () => {
    render(<Select options={options} />)
    expect(screen.getByRole('option', { name: 'Draft' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Staging' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'Production' })).toBeInTheDocument()
  })

  it('shows error message when error prop is provided', () => {
    render(<Select options={options} error="Required" />)
    expect(screen.getByText('Required')).toBeInTheDocument()
  })

  it('shows helperText when no error', () => {
    render(<Select options={options} helperText="Choose an environment" />)
    expect(screen.getByText('Choose an environment')).toBeInTheDocument()
  })

  it('does not show helperText when error is provided', () => {
    render(<Select options={options} error="Required" helperText="Choose" />)
    expect(screen.queryByText('Choose')).not.toBeInTheDocument()
  })

  it('calls onChange when user selects a value', async () => {
    const onChange = jest.fn()
    const user = userEvent.setup()
    render(<Select options={options} onChange={onChange} />)
    await user.selectOptions(screen.getByRole('combobox'), 'staging')
    expect(onChange).toHaveBeenCalled()
  })

  it('applies error border class when error is provided', () => {
    render(<Select options={options} error="Error" />)
    expect(screen.getByRole('combobox').className).toContain('border-red-300')
  })

  it('applies normal border class when no error', () => {
    render(<Select options={options} />)
    expect(screen.getByRole('combobox').className).toContain('border-gray-300')
  })
})
