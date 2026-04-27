import React from 'react'
import { render, screen, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AppContextProvider, useAppContext, isValidStage } from '@/context/AppContext'

// Mock next/navigation
const mockPush = jest.fn()
const mockReplace = jest.fn()
let mockSearchParams = new URLSearchParams()
let mockPathname = '/dashboard'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: mockPush, replace: mockReplace }),
  usePathname: () => mockPathname,
  useSearchParams: () => mockSearchParams,
}))

function TestConsumer() {
  const ctx = useAppContext()
  return (
    <div>
      <div data-testid="application-id">{ctx.applicationId ?? 'null'}</div>
      <div data-testid="stage">{ctx.stage}</div>
      <button onClick={() => ctx.setApplicationId('app-123')}>Set App</button>
      <button onClick={() => ctx.setApplicationId(null)}>Clear App</button>
      <button onClick={() => ctx.setStage('staging')}>Set Staging</button>
      <button onClick={() => ctx.setStage('production')}>Set Production</button>
      <div data-testid="href">{ctx.buildHref('/components')}</div>
    </div>
  )
}

function renderWithProvider() {
  return render(
    <AppContextProvider>
      <TestConsumer />
    </AppContextProvider>
  )
}

describe('isValidStage', () => {
  it('returns true for valid stages', () => {
    expect(isValidStage('draft')).toBe(true)
    expect(isValidStage('staging')).toBe(true)
    expect(isValidStage('production')).toBe(true)
  })

  it('returns false for invalid stages', () => {
    expect(isValidStage('invalid')).toBe(false)
    expect(isValidStage('')).toBe(false)
    expect(isValidStage('DRAFT')).toBe(false)
  })
})

describe('AppContext', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockSearchParams = new URLSearchParams()
    mockPathname = '/dashboard'
  })

  it('throws when useAppContext is used outside AppContextProvider', () => {
    const spy = jest.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => render(<TestConsumer />)).toThrow(
      'useAppContext must be used within AppContextProvider'
    )
    spy.mockRestore()
  })

  it('reads applicationId from URL search params', () => {
    mockSearchParams = new URLSearchParams('application_id=app-42')
    renderWithProvider()
    expect(screen.getByTestId('application-id').textContent).toBe('app-42')
  })

  it('defaults applicationId to null when not in URL', () => {
    renderWithProvider()
    expect(screen.getByTestId('application-id').textContent).toBe('null')
  })

  it('reads stage from URL search params', () => {
    mockSearchParams = new URLSearchParams('stage=staging')
    renderWithProvider()
    expect(screen.getByTestId('stage').textContent).toBe('staging')
  })

  it('defaults stage to draft when not in URL', () => {
    renderWithProvider()
    expect(screen.getByTestId('stage').textContent).toBe('draft')
  })

  it('defaults to draft when stage param is invalid', () => {
    mockSearchParams = new URLSearchParams('stage=invalidstage')
    renderWithProvider()
    expect(screen.getByTestId('stage').textContent).toBe('draft')
  })

  it('setApplicationId calls router.replace with updated params', async () => {
    const user = userEvent.setup()
    renderWithProvider()
    await user.click(screen.getByText('Set App'))
    expect(mockReplace).toHaveBeenCalledWith(
      expect.stringContaining('application_id=app-123'),
      expect.any(Object)
    )
  })

  it('setStage calls router.replace with updated stage', async () => {
    const user = userEvent.setup()
    renderWithProvider()
    await user.click(screen.getByText('Set Staging'))
    expect(mockReplace).toHaveBeenCalledWith(
      expect.stringContaining('stage=staging'),
      expect.any(Object)
    )
  })

  describe('buildHref', () => {
    it('includes application_id and stage in href', () => {
      mockSearchParams = new URLSearchParams('application_id=app-1&stage=staging')
      renderWithProvider()
      const href = screen.getByTestId('href').textContent
      expect(href).toContain('application_id=app-1')
      expect(href).toContain('stage=staging')
    })

    it('builds href with base path', () => {
      mockSearchParams = new URLSearchParams('stage=draft')
      renderWithProvider()
      const href = screen.getByTestId('href').textContent
      expect(href).toMatch(/^\/components\?/)
    })
  })
})
