'use client'

import { useEffect, useState } from 'react'
import { useRouter, useParams } from 'next/navigation'
import Layout from '@/components/Layout'
import { TranslationEditor } from '@/components/TranslationEditor'
import { useAppDispatch, useAppSelector } from '@/hooks/redux'
import { fetchComponent } from '@/store/slices/componentSlice'
import { fetchApplication } from '@/store/slices/applicationSlice'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { ArrowLeft } from 'lucide-react'
import toast from 'react-hot-toast'

export default function ComponentTranslationsPage() {
  const router = useRouter()
  const params = useParams()
  const componentId = params.id as string
  const dispatch = useAppDispatch()
  const { isAuthenticated } = useAppSelector((state) => state.auth)
  const { currentComponent } = useAppSelector((state) => state.components)
  const { currentApplication } = useAppSelector((state) => state.applications)
  const [loading, setLoading] = useState(true)

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
        ])

        if (currentComponent?.application_id) {
          await dispatch(fetchApplication(currentComponent.application_id))
        }
      } catch (error: any) {
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
              onClick={() => router.push('/components')}
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
              onClick={() => router.push('/components')}
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
    </Layout>
  )
}

