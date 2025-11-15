'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import Layout from '@/components/Layout'
import { useAppDispatch, useAppSelector } from '@/hooks/redux'
import { authApi } from '@/services/api'
import toast from 'react-hot-toast'
import { Plus, Edit, UserCheck, UserX } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Table, TableRow, TableCell } from '@/components/ui/Table'
import { Modal } from '@/components/ui/Modal'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { Badge } from '@/components/ui/Badge'

export default function UsersPage() {
  const router = useRouter()
  const dispatch = useAppDispatch()
  const { isAuthenticated, user } = useAppSelector((state) => state.auth)
  const [users, setUsers] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editingUser, setEditingUser] = useState<any>(null)
  const [formData, setFormData] = useState({
    username: '',
    password: '',
    role: 'operator' as 'super_admin' | 'operator' | 'user_manager',
    is_active: true,
  })

  useEffect(() => {
    const token = localStorage.getItem('token')
    if (!token) {
      router.replace('/login')
      return
    }

    // Don't check isAuthenticated immediately - it might not be initialized yet
    // Just check token and proceed with loading
    // Role check will happen when user data is available
    loadUsers()
  }, [router])

  // Check role when user is available
  useEffect(() => {
    if (user && user.role !== 'super_admin' && user.role !== 'user_manager') {
      router.replace('/dashboard')
    }
  }, [user, router])

  const loadUsers = async () => {
    try {
      const data = await authApi.getUsers()
      setUsers(data)
    } catch (error: any) {
      toast.error('Failed to load users')
    } finally {
      setLoading(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      if (editingUser) {
        await authApi.updateUser(editingUser.id, formData)
        toast.success('User updated')
      } else {
        await authApi.createUser(formData)
        toast.success('User created')
      }
      setShowModal(false)
      setEditingUser(null)
      setFormData({
        username: '',
        password: '',
        role: 'operator',
        is_active: true,
      })
      loadUsers()
    } catch (error: any) {
      toast.error(error.response?.data?.error || 'Failed to save user')
    }
  }

  const handleToggleActive = async (userId: string, isActive: boolean) => {
    try {
      await authApi.updateUser(userId, { is_active: !isActive })
      toast.success(`User ${!isActive ? 'activated' : 'deactivated'}`)
      loadUsers()
    } catch (error: any) {
      toast.error('Failed to update user')
    }
  }

  const handleEdit = (userData: any) => {
    setEditingUser(userData)
    setFormData({
      username: userData.username,
      password: '',
      role: userData.role,
      is_active: userData.is_active,
    })
    setShowModal(true)
  }

  if (!isAuthenticated) return null

  const canManage = user?.role === 'super_admin' || user?.role === 'user_manager'

  if (!canManage) return null

  const getRoleBadge = (role: string) => {
    const variants: Record<string, 'primary' | 'success' | 'info'> = {
      super_admin: 'primary',
      operator: 'success',
      user_manager: 'info',
    }
    return (
      <Badge variant={variants[role] || 'info'}>
        {role.replace('_', ' ').toUpperCase()}
      </Badge>
    )
  }

  return (
    <Layout>
      <div className="space-y-6">
        <div className="flex justify-between items-center">
          <h1 className="text-3xl font-bold text-gray-900">User Management</h1>
          {user?.role === 'super_admin' && (
            <Button
              variant="primary"
              onClick={() => {
                setEditingUser(null)
                setFormData({
                  username: '',
                  password: '',
                  role: 'operator',
                  is_active: true,
                })
                setShowModal(true)
              }}
            >
              <Plus className="w-4 h-4 mr-2" />
              New User
            </Button>
          )}
        </div>

        {loading ? (
          <Card>
            <div className="text-center py-8 text-gray-500">Loading...</div>
          </Card>
        ) : (
          <Card>
            <Table headers={['Username', 'Role', 'Status', 'Actions']}>
              {users.map((userData) => (
                <TableRow key={userData.id}>
                  <TableCell>
                    <div className="font-medium text-gray-900">
                      {userData.username}
                    </div>
                  </TableCell>
                  <TableCell>{getRoleBadge(userData.role)}</TableCell>
                  <TableCell>
                    {userData.is_active ? (
                      <Badge variant="success">Active</Badge>
                    ) : (
                      <Badge variant="danger">Inactive</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center space-x-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => handleEdit(userData)}
                      >
                        <Edit className="w-4 h-4" />
                      </Button>
                      <Button
                        variant={userData.is_active ? 'danger' : 'success'}
                        size="sm"
                        onClick={() =>
                          handleToggleActive(userData.id, userData.is_active)
                        }
                      >
                        {userData.is_active ? (
                          <UserX className="w-4 h-4" />
                        ) : (
                          <UserCheck className="w-4 h-4" />
                        )}
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </Table>
          </Card>
        )}

        <Modal
          isOpen={showModal}
          onClose={() => {
            setShowModal(false)
            setEditingUser(null)
          }}
          title={editingUser ? 'Edit User' : 'Create User'}
          footer={
            <>
              <Button variant="primary" onClick={handleSubmit} form="user-form">
                {editingUser ? 'Update' : 'Create'}
              </Button>
              <Button
                variant="outline"
                onClick={() => {
                  setShowModal(false)
                  setEditingUser(null)
                }}
              >
                Cancel
              </Button>
            </>
          }
        >
          <form id="user-form" onSubmit={handleSubmit} className="space-y-4">
            <Input
              label="Username"
              required
              value={formData.username}
              onChange={(e) =>
                setFormData({ ...formData, username: e.target.value })
              }
              disabled={!!editingUser}
            />
            <Input
              label={editingUser ? 'New Password (leave empty to keep current)' : 'Password'}
              type="password"
              required={!editingUser}
              value={formData.password}
              onChange={(e) =>
                setFormData({ ...formData, password: e.target.value })
              }
            />
            <Select
              label="Role"
              required
              value={formData.role}
              onChange={(e) =>
                setFormData({
                  ...formData,
                  role: e.target.value as any,
                })
              }
              options={[
                { value: 'operator', label: 'Operator' },
                { value: 'user_manager', label: 'User Manager' },
                ...(user?.role === 'super_admin'
                  ? [{ value: 'super_admin', label: 'Super Admin' }]
                  : []),
              ]}
            />
            {editingUser && (
              <div className="flex items-center">
                <input
                  type="checkbox"
                  id="is_active"
                  checked={formData.is_active}
                  onChange={(e) =>
                    setFormData({ ...formData, is_active: e.target.checked })
                  }
                  className="h-4 w-4 text-primary-600 focus:ring-primary-500 border-gray-300 rounded"
                />
                <label htmlFor="is_active" className="ml-2 block text-sm text-gray-900">
                  Active
                </label>
              </div>
            )}
          </form>
        </Modal>
      </div>
    </Layout>
  )
}

