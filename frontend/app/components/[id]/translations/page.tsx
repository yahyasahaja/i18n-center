'use client'

import { useEffect, useState } from 'react'
import { useRouter, useParams, useSearchParams } from 'next/navigation'
import Layout from '@/components/Layout'
import { TranslationEditor } from '@/components/TranslationEditor'
import { useAppDispatch, useAppSelector } from '@/hooks/redux'
import { useAppContext } from '@/context/AppContext'
import { fetchComponent } from '@/store/slices/componentSlice'
import { ComponentFormModal } from '@/components/ComponentFormModal'
import { fetchApplication, fetchApplications } from '@/store/slices/applicationSlice'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { ArrowLeft } from 'lucide-react'
import toast from 'react-hot-toast'

export default function ComponentTranslationsPage() {
  const router = useRouter()
  const params = useParams()
  const searchParams = useSearchParams()
  const pathname = `/components/${params.id}/translations`
  const componentId = params.id as string
  const dispatch = useAppDispatch()
  const { push, stage: contextStage } = useAppContext()
  const { isAuthenticated } = useAppSelector((state) => state.auth)
  const { currentComponent } = useAppSelector((state) => state.components)
  const { currentApplication, applications } = useAppSelector((state) => state.applications)
  const [loading, setLoading] = useState(true)
  const [showEditModal, setShowEditModal] = useState(false)

  // Sync URL with global context when missing (so sidebar and page stay in sync)
  useEffect(() => {
    if (!currentComponent) return
    const query = new URLSearchParams(searchParams.toString())
    let changed = false
    if (!query.has('application_id') && currentComponent.application_id) {
      query.set('application_id', currentComponent.application_id)
      changed = true
    }
    if (!query.has('stage') && contextStage) {
      query.set('stage', contextStage)
      changed = true
    }
    if (changed) router.replace(`${pathname}?${query.toString()}`, { scroll: false })
  }, [currentComponent, contextStage, pathname, router, searchParams])

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token) {
      router.replace('/login')
      return
    }

    // Don't check isAuthenticated immediately - it might not be initialized yet
    // Just check token and proceed with loading
    const loadData = async () => {
      try {
        await Promise.all([
          dispatch(fetchComponent(componentId)),
          dispatch(fetchApplications()),
        ])

        if (currentComponent?.application_id) {
          await dispatch(fetchApplication(currentComponent.application_id))
        }
      } catch (error: unknown) {
        toast.error('Failed to load component data')
      } finally {
        setLoading(false)
      }
    }

    if (componentId) {
      loadData()
    }
  }, [componentId, router, dispatch])

  useEffect(() => {
    if (currentComponent?.application_id && !currentApplication) {
      dispatch(fetchApplication(currentComponent.application_id))
    }
  }, [currentComponent, currentApplication, dispatch])

  if (!isAuthenticated || loading) {
    return (
      <Layout>
        <div className="flex items-center justify-center h-64">
          <div className="text-gray-500">Loading...</div>
        </div>
      </Layout>
    )
  }

  if (!currentComponent) {
    return (
      <Layout>
        <Card>
          <div className="text-center py-8">
            <p className="text-gray-500">Component not found</p>
            <Button
              variant="outline"
              onClick={() => push('/components')}
              className="mt-4"
            >
              <ArrowLeft className="w-4 h-4 mr-2" />
              Back to Components
            </Button>
          </div>
        </Card>
      </Layout>
    )
  }

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-4">
            <Button
              variant="outline"
              onClick={() => push('/components')}
            >
              <ArrowLeft className="w-4 h-4 mr-2" />
              Back
            </Button>
            <div>
              <h1 className="text-3xl font-bold text-gray-900">
                {currentComponent.name}
              </h1>
              <p className="text-sm text-gray-500 mt-1">
                {currentApplication?.name} • {currentComponent.description || 'No description'}
                {' · '}
                <button
                  type="button"
                  onClick={() => setShowEditModal(true)}
                  className="text-primary-600 hover:underline font-normal"
                >
                  Edit component (tags & pages)
                </button>
              </p>
            </div>
          </div>
        </div>

        <TranslationEditor
          componentId={componentId}
          componentName={currentComponent.name}
          applicationId={currentComponent.application_id}
          enabledLanguages={currentApplication?.enabled_languages || ['en']}
          defaultLocale={currentComponent.default_locale}
        />
      </div>

      <ComponentFormModal
        isOpen={showEditModal}
        onClose={() => setShowEditModal(false)}
        component={currentComponent}
        applications={
          applications?.length
            ? applications
            : currentApplication
              ? [{ id: currentApplication.id, name: currentApplication.name }]
              : []
        }
        defaultApplicationId={currentComponent?.application_id}
        onSaved={() => {
          dispatch(fetchComponent(componentId))
          setShowEditModal(false)
        }}
      />
    </Layout>
  )
}

