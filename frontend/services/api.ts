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
  getActiveJobs: async (applicationId: string) => {
    const response = await api.get(`/applications/${applicationId}/active-jobs`)
    return response.data as {
      add_language_jobs: Array<{
        job_id: string
        locale: string
        status: string
        total_components: number
        completed_components: number
      }>
      translate_jobs: Array<{
        job_id: string
        component_id: string
        component_code: string
        component_name: string
        job_type: string
        target_locales: string[]
        status: string
      }>
    }
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
  bootstrap: async (
    applicationId: string,
    data: Record<string, unknown>,
    locale: string,
    stage: string
  ): Promise<{
    components_created: number
    components_updated: number
    keys_imported: number
    flat_keys_in_common: number
    components: string[]
  }> => {
    const response = await api.post(
      `/applications/${applicationId}/bootstrap?locale=${encodeURIComponent(locale)}&stage=${encodeURIComponent(stage)}`,
      { data }
    )
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

export interface ComponentListParams {
  applicationId?: string
  search?: string
  page?: number
  pageSize?: number
}

export interface ComponentListResponse {
  data: any[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export const componentApi = {
  getAll: async (params?: ComponentListParams): Promise<ComponentListResponse> => {
    const query: Record<string, any> = {}
    if (params?.applicationId) query.application_id = params.applicationId
    if (params?.search) query.search = params.search
    if (params?.page) query.page = params.page
    if (params?.pageSize) query.page_size = params.pageSize
    const response = await api.get('/components', { params: query })
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
  // Async job endpoints (202-based auto-translate / backfill)
  getTranslateJobStatus: async (jobId: string) => {
    const response = await api.get(`/translate-jobs/${jobId}`)
    return response.data
  },
  listComponentTranslateJobs: async (componentId: string, statusFilter?: string) => {
    const params = statusFilter ? { status: statusFilter } : {}
    const response = await api.get(`/components/${componentId}/translate-jobs`, { params })
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

// ─── CMS Types ────────────────────────────────────────────────────────────────

export interface CmsTemplateField {
  id?: string
  template_id?: string
  key: string
  label: string
  value_type: 'text' | 'textarea' | 'rich_text' | 'json'
  required?: boolean
  sort_order?: number
}

export interface CmsTemplate {
  id: string
  application_id: string
  name: string
  code: string
  description?: string
  fields: CmsTemplateField[]
  created_at: string
  updated_at: string
}

export interface CmsItem {
  id: string
  application_id: string
  template_id: string
  template?: CmsTemplate
  identifier: string
  name: string
  description?: string
  created_at: string
  updated_at: string
}

export interface CmsLocalization {
  id: string
  cms_item_id: string
  locale: string
  stage: string
  version: number
  data: Record<string, any>
  source_locale?: string
  is_active: boolean
  created_at: string
}

// ─── CMS API ──────────────────────────────────────────────────────────────────

export const cmsApi = {
  // Templates
  listTemplates: async (applicationId: string): Promise<CmsTemplate[]> => {
    const response = await api.get(`/applications/${applicationId}/cms/templates`)
    return response.data
  },
  getTemplate: async (id: string): Promise<CmsTemplate> => {
    const response = await api.get(`/cms/templates/${id}`)
    return response.data
  },
  createTemplate: async (applicationId: string, data: Partial<CmsTemplate>): Promise<CmsTemplate> => {
    const response = await api.post(`/applications/${applicationId}/cms/templates`, data)
    return response.data
  },
  updateTemplate: async (id: string, data: Partial<CmsTemplate>): Promise<CmsTemplate> => {
    const response = await api.put(`/cms/templates/${id}`, data)
    return response.data
  },
  deleteTemplate: async (id: string): Promise<void> => {
    await api.delete(`/cms/templates/${id}`)
  },

  // Items
  listItems: async (applicationId: string): Promise<CmsItem[]> => {
    const response = await api.get(`/applications/${applicationId}/cms/items`)
    return response.data
  },
  getItem: async (id: string): Promise<CmsItem> => {
    const response = await api.get(`/cms/items/${id}`)
    return response.data
  },
  createItem: async (applicationId: string, data: Partial<CmsItem>): Promise<CmsItem> => {
    const response = await api.post(`/applications/${applicationId}/cms/items`, data)
    return response.data
  },
  updateItem: async (id: string, data: Partial<CmsItem>): Promise<CmsItem> => {
    const response = await api.put(`/cms/items/${id}`, data)
    return response.data
  },
  deleteItem: async (id: string): Promise<void> => {
    await api.delete(`/cms/items/${id}`)
  },

  // Localizations
  listLocalizations: async (itemId: string): Promise<CmsLocalization[]> => {
    const response = await api.get(`/cms/items/${itemId}/localizations`)
    return response.data
  },
  getLocalization: async (itemId: string, locale: string, stage: string): Promise<CmsLocalization> => {
    const response = await api.get(`/cms/items/${itemId}/localizations/detail`, { params: { locale, stage } })
    return response.data
  },
  saveLocalization: async (itemId: string, locale: string, stage: string, data: Record<string, any>): Promise<CmsLocalization> => {
    const response = await api.post(`/cms/items/${itemId}/localizations`, { locale, stage, data })
    return response.data
  },
  translateLocalization: async (itemId: string, sourceLocale: string, targetLocale: string, stage: string) => {
    const response = await api.post(`/cms/items/${itemId}/localizations/translate`, {
      source_locale: sourceLocale,
      target_locale: targetLocale,
      stage,
    })
    return response.data
  },
  deployLocalization: async (itemId: string, locale: string, fromStage: string, toStage: string): Promise<CmsLocalization> => {
    const response = await api.post(`/cms/items/${itemId}/localizations/deploy`, {
      locale,
      from_stage: fromStage,
      to_stage: toStage,
    })
    return response.data
  },
  revertLocalization: async (itemId: string, locale: string, stage: string, version: number): Promise<CmsLocalization> => {
    const response = await api.post(`/cms/items/${itemId}/localizations/revert`, { locale, stage, version })
    return response.data
  },
  listVersions: async (itemId: string, locale: string, stage: string): Promise<CmsLocalization[]> => {
    const response = await api.get(`/cms/items/${itemId}/localizations/versions`, { params: { locale, stage } })
    return response.data
  },

  // Translate job
  getCmsTranslateJobStatus: async (jobId: string) => {
    const response = await api.get(`/cms/translate-jobs/${jobId}`)
    return response.data
  },

  // Image upload
  uploadImage: async (file: File): Promise<string> => {
    const formData = new FormData()
    formData.append('file', file)
    const response = await api.post('/cms/upload-image', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
    return response.data.url
  },
}

export default api

