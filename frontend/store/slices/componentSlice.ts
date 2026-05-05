import { createSlice, createAsyncThunk } from '@reduxjs/toolkit'
import { componentApi, type ComponentListParams } from '@/services/api'

export interface Component {
  id: string
  application_id: string
  name: string
  code: string
  description: string
  structure: Record<string, any>
  default_locale: string
  tags?: { id: string; application_id: string; code: string }[]
  pages?: { id: string; application_id: string; code: string }[]
}

interface ComponentState {
  components: Component[]
  total: number
  page: number
  pageSize: number
  totalPages: number
  currentComponent: Component | null
  loading: boolean
  error: string | null
}

const initialState: ComponentState = {
  components: [],
  total: 0,
  page: 1,
  pageSize: 20,
  totalPages: 0,
  currentComponent: null,
  loading: false,
  error: null,
}

export const fetchComponents = createAsyncThunk(
  'components/fetchAll',
  async (params?: ComponentListParams) => {
    return await componentApi.getAll(params)
  }
)

export const fetchComponent = createAsyncThunk(
  'components/fetchOne',
  async (id: string) => {
    return await componentApi.getById(id)
  }
)

export const createComponent = createAsyncThunk(
  'components/create',
  async (data: Partial<Component>) => {
    return await componentApi.create(data)
  }
)

export const updateComponent = createAsyncThunk(
  'components/update',
  async ({ id, data }: { id: string; data: Partial<Component> }) => {
    return await componentApi.update(id, data)
  }
)

const componentSlice = createSlice({
  name: 'components',
  initialState,
  reducers: {
    setCurrentComponent: (state, action) => {
      state.currentComponent = action.payload
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchComponents.pending, (state) => {
        state.loading = true
        state.error = null
      })
      .addCase(fetchComponents.fulfilled, (state, action) => {
        state.loading = false
        state.components = action.payload.data
        state.total = action.payload.total
        state.page = action.payload.page
        state.pageSize = action.payload.page_size
        state.totalPages = action.payload.total_pages
      })
      .addCase(fetchComponents.rejected, (state, action) => {
        state.loading = false
        state.error = action.error.message || 'Failed to load components'
      })
      .addCase(fetchComponent.fulfilled, (state, action) => {
        state.currentComponent = action.payload
      })
      .addCase(createComponent.fulfilled, (state, action) => {
        state.components.push(action.payload)
      })
      .addCase(updateComponent.fulfilled, (state, action) => {
        const index = state.components.findIndex(
          (comp) => comp.id === action.payload.id
        )
        if (index !== -1) {
          state.components[index] = action.payload
        }
        if (state.currentComponent?.id === action.payload.id) {
          state.currentComponent = action.payload
        }
      })
  },
})

export default componentSlice.reducer

