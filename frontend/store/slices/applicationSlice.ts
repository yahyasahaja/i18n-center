import { createSlice, createAsyncThunk } from '@reduxjs/toolkit'
import { applicationApi } from '@/services/api'

interface Application {
  id: string
  name: string
  code: string
  description: string
  enabled_languages: string[]
  has_openai_key?: boolean
}

interface ApplicationState {
  applications: Application[]
  currentApplication: Application | null
  loading: boolean
  error: string | null
}

const initialState: ApplicationState = {
  applications: [],
  currentApplication: null,
  loading: false,
  error: null,
}

export const fetchApplications = createAsyncThunk(
  'applications/fetchAll',
  async () => {
    return await applicationApi.getAll()
  }
)

export const fetchApplication = createAsyncThunk(
  'applications/fetchOne',
  async (id: string) => {
    return await applicationApi.getById(id)
  }
)

export const createApplication = createAsyncThunk(
  'applications/create',
  async (data: Partial<Application>) => {
    return await applicationApi.create(data)
  }
)

export const updateApplication = createAsyncThunk(
  'applications/update',
  async ({ id, data }: { id: string; data: Partial<Application> }) => {
    return await applicationApi.update(id, data)
  }
)

const applicationSlice = createSlice({
  name: 'applications',
  initialState,
  reducers: {
    setCurrentApplication: (state, action) => {
      state.currentApplication = action.payload
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchApplications.pending, (state) => {
        state.loading = true
      })
      .addCase(fetchApplications.fulfilled, (state, action) => {
        state.loading = false
        state.applications = action.payload
      })
      .addCase(fetchApplication.fulfilled, (state, action) => {
        state.currentApplication = action.payload
      })
      .addCase(createApplication.fulfilled, (state, action) => {
        state.applications.push(action.payload)
      })
      .addCase(updateApplication.fulfilled, (state, action) => {
        const index = state.applications.findIndex(
          (app) => app.id === action.payload.id
        )
        if (index !== -1) {
          state.applications[index] = action.payload
        }
        if (state.currentApplication?.id === action.payload.id) {
          state.currentApplication = action.payload
        }
      })
  },
})

export default applicationSlice.reducer

