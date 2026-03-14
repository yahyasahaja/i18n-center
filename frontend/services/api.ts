import axios from 'axios'

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'

const api = axios.create({
  baseURL: `${API_URL}/api`,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Request interceptor to add token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// Response interceptor to handle errors
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export const authApi = {
  login: async (credentials: { username: string; password: string }) => {
    const response = await api.post('/auth/login', credentials)
    return response.data
  },
  getCurrentUser: async () => {
    const response = await api.get('/auth/me')
    return response.data
  },
  getUsers: async () => {
    const response = await api.get('/auth/users')
    return response.data
  },
  createUser: async (data: any) => {
    const response = await api.post('/auth/users', data)
    return response.data
  },
  updateUser: async (id: string, data: any) => {
    const response = await api.put(`/auth/users/${id}`, data)
    return response.data
  },
}

export const applicationApi = {
  getAll: async () => {
    const response = await api.get('/applications')
    return response.data
  },
  getById: async (id: string) => {
    const response = await api.get(`/applications/${id}`)
    return response.data
  },
  create: async (data: any) => {
    const response = await api.post('/applications', data)
    return response.data
  },
  update: async (id: string, data: any) => {
    const response = await api.put(`/applications/${id}`, data)
    return response.data
  },
  delete: async (id: string) => {
    const response = await api.delete(`/applications/${id}`)
    return response.data
  },
  addLanguage: async (applicationId: string, data: { locale: string; auto_translate: boolean }) => {
    const response = await api.post(`/applications/${applicationId}/languages`, data)
    return response.data
  },
  getPendingDeploys: async (applicationId: string) => {
    const response = await api.get(`/applications/${applicationId}/pending-deploys`)
    return response.data
  },
  deployLocale: async (applicationId: string, locale: string) => {
    const response = await api.post(`/applications/${applicationId}/deploy-locale`, { locale })
    return response.data
  },
  getAddLanguageJobStatus: async (applicationId: string, jobId: string) => {
    const response = await api.get(`/applications/${applicationId}/jobs/${jobId}`)
    return response.data
  },
  deleteLanguage: async (applicationId: string, locale: string) => {
    const response = await api.delete(`/applications/${applicationId}/languages/${encodeURIComponent(locale)}`)
    return response.data
  },
  listApiKeys: async (applicationId: string) => {
    const response = await api.get(`/applications/${applicationId}/api-keys`)
    return response.data
  },
  createApiKey: async (applicationId: string, data?: { name?: string }) => {
    const response = await api.post(`/applications/${applicationId}/api-keys`, data ?? {})
    return response.data
  },
  deleteApiKey: async (applicationId: string, keyId: string) => {
    const response = await api.delete(`/applications/${applicationId}/api-keys/${keyId}`)
    return response.data
  },
}

export type ApplicationAPIKey = { id: string; key_prefix: string; name: string; created_at: string }

export type Tag = { id: string; application_id: string; code: string }
export type Page = { id: string; application_id: string; code: string }

export const tagApi = {
  listByApplication: async (applicationId: string) => {
    const response = await api.get(`/applications/${applicationId}/tags`)
    return response.data as Tag[]
  },
  get: async (id: string) => {
    const response = await api.get(`/tags/${id}`)
    return response.data as Tag
  },
  create: async (applicationId: string, data: { code: string }) => {
    const response = await api.post(`/applications/${applicationId}/tags`, data)
    return response.data as Tag
  },
  update: async (id: string, data: { code?: string }) => {
    const response = await api.put(`/tags/${id}`, data)
    return response.data as Tag
  },
  delete: async (id: string) => {
    const response = await api.delete(`/tags/${id}`)
    return response.data
  },
  getComponents: async (tagId: string) => {
    const response = await api.get(`/tags/${tagId}/components`)
    return response.data
  },
}

export const pageApi = {
  listByApplication: async (applicationId: string) => {
    const response = await api.get(`/applications/${applicationId}/pages`)
    return response.data as Page[]
  },
  get: async (id: string) => {
    const response = await api.get(`/pages/${id}`)
    return response.data as Page
  },
  create: async (applicationId: string, data: { code: string }) => {
    const response = await api.post(`/applications/${applicationId}/pages`, data)
    return response.data as Page
  },
  update: async (id: string, data: { code?: string }) => {
    const response = await api.put(`/pages/${id}`, data)
    return response.data as Page
  },
  delete: async (id: string) => {
    const response = await api.delete(`/pages/${id}`)
    return response.data
  },
  getComponents: async (pageId: string) => {
    const response = await api.get(`/pages/${pageId}/components`)
    return response.data
  },
}

export const componentApi = {
  getAll: async (applicationId?: string) => {
    const params = applicationId ? { application_id: applicationId } : {}
    const response = await api.get('/components', { params })
    return response.data
  },
  getById: async (id: string) => {
    const response = await api.get(`/components/${id}`)
    return response.data
  },
  create: async (data: any) => {
    const response = await api.post('/components', data)
    return response.data
  },
  update: async (id: string, data: any) => {
    const response = await api.put(`/components/${id}`, data)
    return response.data
  },
  delete: async (id: string) => {
    const response = await api.delete(`/components/${id}`)
    return response.data
  },
}

export const translationApi = {
  get: async (componentId: string, locale: string, stage: string) => {
    const response = await api.get(
      `/components/${componentId}/translations?locale=${locale}&stage=${stage}`
    )
    return response.data
  },
  save: async (
    componentId: string,
    locale: string,
    stage: string,
    data: any
  ) => {
    const response = await api.post(`/components/${componentId}/translations`, {
      locale,
      stage,
      data,
    })
    return response.data
  },
  revert: async (componentId: string, locale: string, stage: string) => {
    const response = await api.post(
      `/components/${componentId}/translations/revert?locale=${locale}&stage=${stage}`
    )
    return response.data
  },
  deploy: async (
    componentId: string,
    locale: string,
    fromStage: string,
    toStage: string
  ) => {
    const response = await api.post(
      `/components/${componentId}/translations/deploy`,
      { locale, from_stage: fromStage, to_stage: toStage }
    )
    return response.data
  },
  autoTranslate: async (
    componentId: string,
    sourceLocale: string,
    targetLocale: string,
    stage: string
  ) => {
    const response = await api.post(
      `/components/${componentId}/translations/auto-translate`,
      { source_locale: sourceLocale, target_locale: targetLocale, stage }
    )
    return response.data
  },
  backfill: async (
    componentId: string,
    sourceLocale: string,
    targetLocales: string[],
    stage: string
  ) => {
    const response = await api.post(
      `/components/${componentId}/translations/backfill`,
      { source_locale: sourceLocale, target_locales: targetLocales, stage }
    )
    return response.data
  },
  compare: async (componentId: string, locale: string, stage: string, versionA?: number, versionB?: number) => {
    let url = `/components/${componentId}/translations/compare?locale=${locale}&stage=${stage}`
    if (versionA != null && versionB != null) {
      url += `&version_a=${versionA}&version_b=${versionB}`
    }
    const response = await api.get(url)
    return response.data
  },
  listVersions: async (componentId: string, locale: string, stage: string) => {
    const response = await api.get(
      `/components/${componentId}/translations/versions?locale=${locale}&stage=${stage}`
    )
    return response.data
  },
}

export const exportApi = {
  exportApplication: async (
    applicationId: string,
    locale?: string,
    stage?: string
  ) => {
    const params: any = {}
    if (locale) params.locale = locale
    if (stage) params.stage = stage
    const response = await api.get(`/applications/${applicationId}/export`, {
      params,
      responseType: 'blob',
    })
    return response.data
  },
  exportComponent: async (
    componentId: string,
    locale?: string,
    stage?: string
  ) => {
    const params: any = {}
    if (locale) params.locale = locale
    if (stage) params.stage = stage
    const response = await api.get(`/components/${componentId}/export`, {
      params,
      responseType: 'blob',
    })
    return response.data
  },
}

export const importApi = {
  importComponent: async (
    componentId: string,
    locale: string,
    stage: string,
    data: any
  ) => {
    const response = await api.post(
      `/components/${componentId}/import?locale=${locale}&stage=${stage}`,
      { data }
    )
    return response.data
  },
}

export default api

