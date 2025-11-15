import { createSlice, createAsyncThunk } from '@reduxjs/toolkit'
import { translationApi } from '@/services/api'

interface Translation {
  id: string
  component_id: string
  locale: string
  stage: string
  data: Record<string, any>
}

interface TranslationState {
  translations: Translation[]
  currentTranslation: Translation | null
  loading: boolean
  error: string | null
}

const initialState: TranslationState = {
  translations: [],
  currentTranslation: null,
  loading: false,
  error: null,
}

export const fetchTranslation = createAsyncThunk(
  'translations/fetchOne',
  async ({
    componentId,
    locale,
    stage,
  }: {
    componentId: string
    locale: string
    stage: string
  }) => {
    return await translationApi.get(componentId, locale, stage)
  }
)

export const saveTranslation = createAsyncThunk(
  'translations/save',
  async ({
    componentId,
    locale,
    stage,
    data,
  }: {
    componentId: string
    locale: string
    stage: string
    data: Record<string, any>
  }) => {
    return await translationApi.save(componentId, locale, stage, data)
  }
)

const translationSlice = createSlice({
  name: 'translations',
  initialState,
  reducers: {
    setCurrentTranslation: (state, action) => {
      state.currentTranslation = action.payload
    },
  },
  extraReducers: (builder) => {
    builder
      .addCase(fetchTranslation.pending, (state) => {
        state.loading = true
      })
      .addCase(fetchTranslation.fulfilled, (state, action) => {
        state.loading = false
        state.currentTranslation = action.payload
      })
      .addCase(saveTranslation.fulfilled, (state, action) => {
        state.currentTranslation = action.payload
      })
  },
})

export default translationSlice.reducer

