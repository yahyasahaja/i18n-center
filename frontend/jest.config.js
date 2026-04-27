const nextJest = require('next/jest')

const createJestConfig = nextJest({ dir: './' })

module.exports = createJestConfig({
  setupFilesAfterEnv: ['<rootDir>/jest.setup.ts'],
  testEnvironment: 'jest-environment-jsdom',
  moduleNameMapper: {
    '^@/(.*)$': '<rootDir>/$1',
    '\\.(css|less|scss|sass)$': 'identity-obj-proxy',
  },
  testMatch: ['**/__tests__/**/*.test.{ts,tsx}'],
  collectCoverage: true,
  collectCoverageFrom: [
    'components/ui/**/*.{ts,tsx}',
    'store/slices/**/*.{ts,tsx}',
    'context/**/*.{ts,tsx}',
    'services/api.ts',
    '!**/*.d.ts',
  ],
  coverageThreshold: {
    global: { lines: 80 },
  },
})
