import React from 'react'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Table, TableRow, TableCell } from '@/components/ui/Table'

describe('Table', () => {
  it('renders column headers', () => {
    render(<Table headers={['Name', 'Code', 'Actions']}>{null}</Table>)
    expect(screen.getByText('Name')).toBeInTheDocument()
    expect(screen.getByText('Code')).toBeInTheDocument()
    expect(screen.getByText('Actions')).toBeInTheDocument()
  })

  it('renders children inside tbody', () => {
    render(
      <Table headers={['Name']}>
        <tr>
          <td>Row content</td>
        </tr>
      </Table>
    )
    expect(screen.getByText('Row content')).toBeInTheDocument()
  })

  it('renders correct number of header columns', () => {
    render(<Table headers={['A', 'B', 'C', 'D']}>{null}</Table>)
    expect(screen.getAllByRole('columnheader')).toHaveLength(4)
  })
})

describe('TableRow', () => {
  it('renders children', () => {
    render(
      <table>
        <tbody>
          <TableRow>
            <td>Cell content</td>
          </TableRow>
        </tbody>
      </table>
    )
    expect(screen.getByText('Cell content')).toBeInTheDocument()
  })

  it('calls onClick when clicked and onClick is provided', async () => {
    const onClick = jest.fn()
    const user = userEvent.setup()
    render(
      <table>
        <tbody>
          <TableRow onClick={onClick}>
            <td>Clickable row</td>
          </TableRow>
        </tbody>
      </table>
    )
    await user.click(screen.getByText('Clickable row'))
    expect(onClick).toHaveBeenCalledTimes(1)
  })

  it('applies cursor-pointer class when onClick is provided', () => {
    const { container } = render(
      <table>
        <tbody>
          <TableRow onClick={jest.fn()}>
            <td>Row</td>
          </TableRow>
        </tbody>
      </table>
    )
    expect(container.querySelector('tr')!.className).toContain('cursor-pointer')
  })

  it('does not apply cursor-pointer when onClick is not provided', () => {
    const { container } = render(
      <table>
        <tbody>
          <TableRow>
            <td>Row</td>
          </TableRow>
        </tbody>
      </table>
    )
    expect(container.querySelector('tr')!.className).not.toContain('cursor-pointer')
  })
})

describe('TableCell', () => {
  it('renders children content', () => {
    render(
      <table>
        <tbody>
          <tr>
            <TableCell>Cell value</TableCell>
          </tr>
        </tbody>
      </table>
    )
    expect(screen.getByText('Cell value')).toBeInTheDocument()
  })

  it('applies custom className', () => {
    const { container } = render(
      <table>
        <tbody>
          <tr>
            <TableCell className="custom-cell">Content</TableCell>
          </tr>
        </tbody>
      </table>
    )
    expect(container.querySelector('td')!.className).toContain('custom-cell')
  })
})
