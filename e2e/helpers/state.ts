import * as fs   from 'fs'
import * as path from 'path'

const STATE_FILE = path.join(__dirname, '../.test-state.json')

export interface TestState {
  token:           string
  applicationId:   string
  applicationCode: string
  componentId:     string
  componentCode:   string
  tagId:           string
  pageId:          string
  apiKey:          string
  apiKeyId:        string
  secondLocale:    string  // language enabled in addition to EN, used by translate/backfill tests
}

export function saveState(state: TestState): void {
  fs.mkdirSync(path.dirname(STATE_FILE), { recursive: true })
  fs.writeFileSync(STATE_FILE, JSON.stringify(state, null, 2))
}

export function loadState(): TestState {
  return JSON.parse(fs.readFileSync(STATE_FILE, 'utf-8')) as TestState
}

export function stateExists(): boolean {
  return fs.existsSync(STATE_FILE)
}
