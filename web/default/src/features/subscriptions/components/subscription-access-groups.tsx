/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useEffectEvent, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/design-system/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/design-system/dialog'
import { Input } from '@/components/design-system/input'
import { Switch } from '@/components/ui/switch'

import {
  createSubscriptionAccessGroup,
  deleteSubscriptionAccessGroup,
  getSubscriptionAccessGroups,
  getSubscriptionAccessGroupPlans,
  getSubscriptionAccessGroupUsers,
  removePlanFromSubscriptionAccessGroup,
  removeUserFromSubscriptionAccessGroup,
  updateSubscriptionAccessGroup,
} from '../api'
import type {
  SubscriptionAccessGroup,
  SubscriptionAccessGroupPlan,
  SubscriptionAccessGroupUser,
} from '../types'

type AccessGroupDraft = Pick<SubscriptionAccessGroup, 'name' | 'description' | 'enabled' | 'sort_order'>

const EMPTY_DRAFT: AccessGroupDraft = {
	name: '',
  description: '',
  enabled: true,
  sort_order: 0,
}

export function SubscriptionAccessGroups() {
  const { t } = useTranslation()
  const [groups, setGroups] = useState<SubscriptionAccessGroup[]>([])
  const [draft, setDraft] = useState<AccessGroupDraft>(EMPTY_DRAFT)
  const [submitting, setSubmitting] = useState(false)
  const [detailsGroup, setDetailsGroup] = useState<SubscriptionAccessGroup | null>(null)
  const [detailsType, setDetailsType] = useState<'users' | 'plans'>('users')
  const [detailUsers, setDetailUsers] = useState<SubscriptionAccessGroupUser[]>([])
  const [detailPlans, setDetailPlans] = useState<SubscriptionAccessGroupPlan[]>([])

  const reload = useEffectEvent(async () => {
    try {
      const res = await getSubscriptionAccessGroups()
      if (res.success) setGroups(res.data || [])
    } catch {
      toast.error(t('Request failed'))
    }
  })

  useEffect(() => {
    void reload()
  }, [])

  const create = async () => {
	if (!draft.name.trim()) return
    setSubmitting(true)
    try {
      const res = await createSubscriptionAccessGroup({
        ...draft,
        name: draft.name.trim(),
        description: draft.description.trim(),
      })
      if (!res.success) {
        toast.error(res.message || t('Create failed'))
        return
      }
      setDraft(EMPTY_DRAFT)
      await reload()
    } finally {
      setSubmitting(false)
    }
  }

  const update = async (group: SubscriptionAccessGroup) => {
    const res = await updateSubscriptionAccessGroup(group.id, group)
    if (!res.success) {
      toast.error(res.message || t('Update failed'))
      return
    }
    await reload()
  }

  const remove = async (group: SubscriptionAccessGroup) => {
    if (!window.confirm(t('Are you sure you want to delete this item?'))) return
    const res = await deleteSubscriptionAccessGroup(group.id)
    if (!res.success) {
      toast.error(res.message || t('Delete failed'))
      return
    }
    await reload()
  }

  const showDetails = async (group: SubscriptionAccessGroup, type: 'users' | 'plans') => {
    setDetailsGroup(group)
    setDetailsType(type)
    if (type === 'users') {
      const res = await getSubscriptionAccessGroupUsers(group.id)
      if (res.success) setDetailUsers(res.data || [])
      return
    }
    const res = await getSubscriptionAccessGroupPlans(group.id)
    if (res.success) setDetailPlans(res.data || [])
  }

  const removeUser = async (user: SubscriptionAccessGroupUser) => {
    if (!detailsGroup || !window.confirm(t('Are you sure you want to delete this item?'))) return
    const res = await removeUserFromSubscriptionAccessGroup(detailsGroup.id, user.id)
    if (!res.success) {
      toast.error(res.message || t('Delete failed'))
      return
    }
    setDetailUsers((users) => users.filter((item) => item.id !== user.id))
    await reload()
  }

  const removePlan = async (plan: SubscriptionAccessGroupPlan) => {
    if (!detailsGroup || !window.confirm(t('Are you sure you want to delete this item?'))) return
    const res = await removePlanFromSubscriptionAccessGroup(detailsGroup.id, plan.id)
    if (!res.success) {
      toast.error(res.message || t('Delete failed'))
      return
    }
    setDetailPlans((plans) => plans.filter((item) => item.id !== plan.id))
    await reload()
  }

  return (
    <div className='space-y-4'>
      <div className='grid gap-3 rounded-lg border p-4 sm:grid-cols-[1fr_1fr_auto]'>
        <Input placeholder={t('Permission Name')} value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
        <Input placeholder={t('Description')} value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} />
        <Button onClick={create} disabled={submitting || !draft.name.trim()}>
          <Plus />
          {t('Create')}
        </Button>
      </div>
      <div className='overflow-x-auto rounded-lg border'>
        <table className='w-full min-w-[720px] text-sm'>
          <thead className='bg-muted/50 text-left text-muted-foreground'>
            <tr>
              <th className='p-3'>{t('ID')}</th>
              <th className='p-3'>{t('Permission Name')}</th>
              <th className='p-3'>{t('Description')}</th>
              <th className='p-3'>{t('Status')}</th>
              <th className='p-3'>{t('Users')}</th>
              <th className='p-3'>{t('Plans')}</th>
              <th className='p-3'>{t('Actions')}</th>
            </tr>
          </thead>
          <tbody>
            {groups.map((group) => (
              <tr key={group.id} className='border-t'>
                <td className='p-3'>{group.id}</td>
                <td className='p-3 font-medium'>{group.name}{group.is_default ? ` (${t('Default')})` : ''}</td>
                <td className='p-3 text-muted-foreground'>{group.description || '-'}</td>
                <td className='p-3'><Switch checked={group.enabled} disabled={group.is_default} onCheckedChange={(enabled) => void update({ ...group, enabled })} /></td>
                <td className='p-3'><Button variant='link' className='h-auto p-0' onClick={() => void showDetails(group, 'users')}>{group.user_count ?? 0}</Button></td>
                <td className='p-3'><Button variant='link' className='h-auto p-0' onClick={() => void showDetails(group, 'plans')}>{group.plan_count ?? 0}</Button></td>
                <td className='p-3'><Button variant='ghost' size='icon' disabled={group.is_default} onClick={() => void remove(group)} aria-label={t('Delete')}><Trash2 /></Button></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <Dialog open={detailsGroup !== null} onOpenChange={(open) => !open && setDetailsGroup(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{detailsGroup?.name} · {detailsType === 'users' ? t('Users') : t('Plans')}</DialogTitle>
          </DialogHeader>
          {detailsType === 'users' && (
            detailUsers.length === 0 ? <p className='text-muted-foreground'>{t('No data')}</p> : (
              <div className='space-y-2'>{detailUsers.map((user) => <div key={user.id} className='flex items-center justify-between gap-2'><span><span className='font-medium'>{user.username}</span>{user.display_name ? ` (${user.display_name})` : ''}</span><Button variant='ghost' size='icon-sm' disabled={detailsGroup?.is_default} onClick={() => void removeUser(user)} aria-label={t('Delete')}><Trash2 /></Button></div>)}</div>
            )
          )}
          {detailsType === 'plans' && (
            detailPlans.length === 0 ? <p className='text-muted-foreground'>{t('No data')}</p> : (
              <div className='space-y-2'>{detailPlans.map((plan) => <div key={plan.id} className='flex items-center justify-between gap-2'><span><span className='font-medium'>{plan.title}</span>{!plan.enabled ? ` (${t('Disabled')})` : ''}</span><Button variant='ghost' size='icon-sm' disabled={detailsGroup?.is_default} onClick={() => void removePlan(plan)} aria-label={t('Delete')}><Trash2 /></Button></div>)}</div>
            )
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
