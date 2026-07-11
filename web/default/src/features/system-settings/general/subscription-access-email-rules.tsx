import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/design-system/button'
import { Dialog } from '@/components/dialog'
import { Input } from '@/components/design-system/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import {
  getSubscriptionAccessEmailRules,
  getSubscriptionAccessGroups,
  replaceSubscriptionAccessEmailRules,
} from '@/features/subscriptions/api'
import type { SubscriptionAccessEmailRule } from '@/features/subscriptions/types'

type RuleDraft = Pick<SubscriptionAccessEmailRule, 'email' | 'group_ids'>

const emptyDraft: RuleDraft = { email: '', group_ids: [] }

export function SubscriptionAccessEmailRules() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingIndex, setEditingIndex] = useState<number | null>(null)
  const [draft, setDraft] = useState<RuleDraft>(emptyDraft)
  const { data: rulesData, isLoading } = useQuery({
    queryKey: ['subscription-access-email-rules'],
    queryFn: getSubscriptionAccessEmailRules,
  })
  const { data: groupsData } = useQuery({
    queryKey: ['subscription-access-groups'],
    queryFn: getSubscriptionAccessGroups,
  })
  const rules = rulesData?.data || []
  const groups = groupsData?.data || []
  const saveRules = useMutation({
    mutationFn: replaceSubscriptionAccessEmailRules,
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: ['subscription-access-email-rules'],
      })
      toast.success(t('Saved successfully'))
      setDialogOpen(false)
    },
    onError: () => toast.error(t('Request failed')),
  })

  const openCreate = () => {
    setEditingIndex(null)
    setDraft(emptyDraft)
    setDialogOpen(true)
  }

  const openEdit = (index: number) => {
    const rule = rules[index]
    setEditingIndex(index)
    setDraft({ email: rule.email, group_ids: rule.group_ids })
    setDialogOpen(true)
  }

  const submitDraft = () => {
    const email = draft.email.trim().toLowerCase()
    if (!email || !email.includes('@')) {
      toast.error(t('Please enter a valid email address'))
      return
    }
    const nextRules = rules.map((rule) => ({
      email: rule.email,
      group_ids: rule.group_ids,
    }))
    const duplicateIndex = nextRules.findIndex(
      (rule, index) => rule.email.toLowerCase() === email && index !== editingIndex
    )
    if (duplicateIndex !== -1) {
      toast.error(t('An email rule already exists for this address'))
      return
    }
    const nextRule = { email, group_ids: draft.group_ids }
    if (editingIndex === null) {
      nextRules.push(nextRule)
    } else {
      nextRules[editingIndex] = nextRule
    }
    saveRules.mutate(nextRules)
  }

  const deleteRule = (index: number) => {
    saveRules.mutate(
      rules
        .filter((_, ruleIndex) => ruleIndex !== index)
        .map((rule) => ({ email: rule.email, group_ids: rule.group_ids }))
    )
  }

  const toggleGroup = (groupId: number, checked: boolean) => {
    setDraft((current) => ({
      ...current,
      group_ids: checked
        ? [...current.group_ids, groupId]
        : current.group_ids.filter((id) => id !== groupId),
    }))
  }

  const groupNames = (groupIds: number[]) => {
    const names = groups
      .filter((group) => groupIds.includes(group.id))
      .map((group) => group.name)
    return names.length > 0 ? names.join(', ') : t('Default access group')
  }

  let ruleList: ReactNode
  if (isLoading) {
    ruleList = <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
  } else if (rules.length === 0) {
    ruleList = <p className='text-muted-foreground text-sm'>{t('No email rules configured')}</p>
  } else {
    ruleList = rules.map((rule, index) => (
      <div
        key={rule.email}
        className='flex flex-col gap-2 rounded-md border p-3 sm:flex-row sm:items-center sm:justify-between'
      >
        <div className='min-w-0'>
          <p className='truncate font-medium'>{rule.email}</p>
          <p className='text-muted-foreground text-sm'>{groupNames(rule.group_ids)}</p>
        </div>
        <div className='flex gap-2'>
          <Button
            type='button'
            size='icon'
            variant='ghost'
            aria-label={t('Edit')}
            onClick={() => openEdit(index)}
          >
            <Pencil className='size-4' />
            <span className='sr-only'>{t('Edit')}</span>
          </Button>
          <Button
            type='button'
            size='icon'
            variant='ghost'
            aria-label={t('Delete')}
            disabled={saveRules.isPending}
            onClick={() => deleteRule(index)}
          >
            <Trash2 className='size-4 text-destructive' />
            <span className='sr-only'>{t('Delete')}</span>
          </Button>
        </div>
      </div>
    ))
  }

  return (
    <div className='rounded-lg border p-4'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
        <div>
          <h3 className='font-medium'>{t('New user subscription visibility rules')}</h3>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t('Assign subscription access groups when a new user registers with a matching email address.')}
          </p>
        </div>
        <Button type='button' variant='outline' onClick={openCreate}>
          <Plus className='mr-2 size-4' />
          {t('Add rule')}
        </Button>
      </div>

      <div className='mt-4 space-y-2'>
        {ruleList}
      </div>

      <Dialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        title={editingIndex === null ? t('Add email rule') : t('Edit email rule')}
        description={t('Choose the subscription access groups assigned to future matching registrations.')}
        footer={
          <div className='flex justify-end gap-2'>
            <Button type='button' variant='outline' onClick={() => setDialogOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button type='button' disabled={saveRules.isPending} onClick={submitDraft}>
              {t('Save')}
            </Button>
          </div>
        }
      >
        <div className='space-y-4'>
          <div className='space-y-2'>
            <Label htmlFor='subscription-access-rule-email'>{t('Email')}</Label>
            <Input
              id='subscription-access-rule-email'
              type='email'
              value={draft.email}
              onChange={(event) =>
                setDraft((current) => ({ ...current, email: event.target.value }))
              }
              placeholder='user@example.com'
            />
          </div>
          <div className='space-y-2'>
            <Label>{t('Subscription visibility permissions')}</Label>
            <div className='space-y-2 rounded-md border p-3'>
              {groups.filter((group) => group.enabled).map((group) => (
                <label key={group.id} className='flex cursor-pointer items-center gap-2 text-sm'>
                  <Checkbox
                    checked={draft.group_ids.includes(group.id)}
                    onCheckedChange={(checked) => toggleGroup(group.id, checked === true)}
                  />
                  <span>{group.name}</span>
                </label>
              ))}
            </div>
            <p className='text-muted-foreground text-xs'>
              {t('Leave all groups unselected to use the default access group.')}
            </p>
          </div>
        </div>
      </Dialog>
    </div>
  )
}
