import { configureStore } from '@reduxjs/toolkit'
import authReducer from './slices/authSlice'
import applicationReducer from './slices/applicationSlice'
import componentReducer from './slices/componentSlice'
import translationReducer from './slices/translationSlice'

export const store = configureStore({
  reducer: {
    auth: authReducer,
    applications: applicationReducer,
    components: componentReducer,
    translations: translationReducer,
  },
})

export type RootState = ReturnType<typeof store.getState>
export type AppDispatch = typeof store.dispatch

