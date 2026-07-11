/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useState, type ChangeEvent } from 'react'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Input } from '@/components/design-system/input'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/design-system/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/design-system/select'
import { getCurrencyDisplay } from '@/lib/currency'
import { getGroups } from '@/features/users/api'
import { formatQuota } from '@/lib/format'

import { SubscriptionAccessEmailRules } from './subscription-access-email-rules'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
  SettingsFormGrid,
  SettingsFormGridItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const quotaSchema = z.object({
  QuotaForNewUser: z.coerce.number().min(0),
  DefaultGroupForNewUser: z.string().min(1),
  PreConsumedQuota: z.coerce.number().min(0),
  QuotaForInviter: z.coerce.number().min(0),
  QuotaForInvitee: z.coerce.number().min(0),
  TopUpLink: z.string(),
  general_setting: z.object({
    docs_link: z.string(),
  }),
  quota_setting: z.object({
    enable_free_model_pre_consume: z.boolean(),
    auto_subscribe_free_plans_for_new_user: z.boolean(),
  }),
})

type QuotaFormValues = z.infer<typeof quotaSchema>
type QuotaInputValue = number | ''

function formatQuotaInputValue(value: QuotaInputValue): string {
  return formatQuota(value === '' ? 0 : value)
}

type QuotaSettingsSectionProps = {
  defaultValues: QuotaFormValues
  complianceConfirmed?: boolean
}

export function QuotaSettingsSection({
  defaultValues,
  complianceConfirmed = true,
}: QuotaSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const { data: groupsData } = useQuery({
    queryKey: ['groups'],
    queryFn: getGroups,
  })
  const groups = groupsData?.data || []
  const [usdConverterOpen, setUsdConverterOpen] = useState(false)
  const [usdAmount, setUsdAmount] = useState('')
  const handleNumberChange =
    (onChange: (value: QuotaInputValue) => void) =>
    (event: ChangeEvent<HTMLInputElement>) => {
      const value = event.currentTarget.valueAsNumber
      onChange(Number.isNaN(value) ? '' : value)
    }

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<QuotaFormValues>({
      resolver: zodResolver(quotaSchema) as Resolver<
        QuotaFormValues,
        unknown,
        QuotaFormValues
      >,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          await updateOption.mutateAsync({
            key,
            value: value as string | number | boolean,
          })
        }
      },
    })
  const parsedUsdAmount = Number.parseFloat(usdAmount)
  const convertedQuota = Number.isFinite(parsedUsdAmount)
    ? Math.round(parsedUsdAmount * getCurrencyDisplay().config.quotaPerUnit)
    : 0

  return (
    <SettingsSection title={t('Quota Settings')}>
      <FormNavigationGuard when={isDirty} />

      {!complianceConfirmed ? (
        <Alert variant='destructive'>
          <AlertDescription>
            {t(
              'Non-zero invitation rewards require compliance confirmation in Payment Gateway settings.'
            )}
          </AlertDescription>
        </Alert>
      ) : null}

      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='QuotaForNewUser'
              render={({ field }) => (
                <FormItem>
                  <div className='flex items-center justify-between gap-2'>
                    <FormLabel>{t('New User Quota')}</FormLabel>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() => setUsdConverterOpen(true)}
                    >
                      {t('Convert USD to Quota')}
                    </Button>
                  </div>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Initial quota given to new users ({{formattedQuota}})',
                      {
                        formattedQuota: formatQuotaInputValue(field.value),
                      }
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='DefaultGroupForNewUser'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Default Group for New Users')}</FormLabel>
                  <Select
                    items={groups.map((group) => ({ value: group, label: group }))}
                    value={field.value}
                    onValueChange={field.onChange}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue placeholder={t('Select a group')} />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      {groups.map((group) => (
                        <SelectItem key={group} value={group}>
                          {group}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t('Newly registered users are assigned to this group by default.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='PreConsumedQuota'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Pre-Consumed Quota')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Quota consumed before charging users')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='QuotaForInviter'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Inviter Reward')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Quota given to users who invite others ({{formattedQuota}})',
                      {
                        formattedQuota: formatQuotaInputValue(field.value),
                      }
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='QuotaForInvitee'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Invitee Reward')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Quota given to invited users ({{formattedQuota}})', {
                      formattedQuota: formatQuotaInputValue(field.value),
                    })}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='quota_setting.auto_subscribe_free_plans_for_new_user'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Auto-Subscribe New Users to Free Plans')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Automatically subscribe new users to enabled free plans they can access.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />

              <FormField
                control={form.control}
                name='quota_setting.enable_free_model_pre_consume'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Pre-Consume for Free Models')}</FormLabel>
                      <FormDescription>
                        {t(
                          'When enabled, zero-cost models also pre-consume quota before final settlement.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
            </SettingsFormGridItem>

            <FormField
              control={form.control}
              name='TopUpLink'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Top-Up Link')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('https://example.com/topup')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('External link for users to purchase quota')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='general_setting.docs_link'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Documentation Link')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('https://docs.example.com')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Link to your documentation site')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>

          <Dialog
            open={usdConverterOpen}
            onOpenChange={setUsdConverterOpen}
            title={t('Convert USD to Quota')}
            description={t('Enter a USD amount to calculate the new user quota.')}
            contentHeight='auto'
            footer={
              <>
                <Button
                  type='button'
                  variant='outline'
                  onClick={() => setUsdConverterOpen(false)}
                >
                  {t('Cancel')}
                </Button>
                <Button
                  type='button'
                  disabled={!Number.isFinite(parsedUsdAmount) || parsedUsdAmount < 0}
                  onClick={() => {
                    form.setValue('QuotaForNewUser', convertedQuota, {
                      shouldDirty: true,
                      shouldValidate: true,
                    })
                    setUsdConverterOpen(false)
                    setUsdAmount('')
                  }}
                >
                  {t('Confirm')}
                </Button>
              </>
            }
          >
            <div className='space-y-3'>
              <Label htmlFor='new-user-quota-usd'>{t('USD Amount')}</Label>
              <Input
                id='new-user-quota-usd'
                type='number'
                min='0'
                step='any'
                placeholder={t('Enter USD amount')}
                value={usdAmount}
                onChange={(event) => setUsdAmount(event.target.value)}
              />
              <p className='text-muted-foreground text-sm'>
                {t('Converted quota: {{quota}}', {
                  quota: convertedQuota.toLocaleString(),
                })}
              </p>
            </div>
          </Dialog>
        </SettingsForm>
      </Form>
      <SubscriptionAccessEmailRules />
    </SettingsSection>
  )
}
