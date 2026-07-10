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
import type { Table } from '@tanstack/react-table'
import { Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { Button } from '@/components/ui/button'

import { deleteRedemption } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import { useRedemptions } from './redemptions-provider'
import type { Redemption } from '../types'

type DataTableBulkActionsProps<TData> = {
  table: Table<TData>
}

export function DataTableBulkActions<TData>({
  table,
}: DataTableBulkActionsProps<TData>) {
  const { t } = useTranslation()
  const { triggerRefresh } = useRedemptions()
  const [isDeleting, setIsDeleting] = useState(false)
  const selectedRows = table.getSelectedRowModel().rows

  const contentToCopy = useMemo(() => {
    const selectedCodes = selectedRows.map((row) => {
      const redemption = row.original as Redemption
      return `${redemption.name}\t${redemption.key}`
    })
    return selectedCodes.join('\n')
  }, [selectedRows])

  const handleBulkDelete = async () => {
    const ids = selectedRows.map((row) => (row.original as Redemption).id)
    if (ids.length === 0) return
    if (!window.confirm(t('Are you sure you want to delete the selected redemption codes?'))) return
    setIsDeleting(true)
    try {
      const results = await Promise.all(ids.map((id) => deleteRedemption(id)))
      const failed = results.filter((res) => !res.success)
      if (failed.length > 0) {
        toast.error(failed[0].message || t(ERROR_MESSAGES.DELETE_FAILED))
      } else {
        toast.success(t(SUCCESS_MESSAGES.REDEMPTION_DELETED))
        table.resetRowSelection()
        triggerRefresh()
      }
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <BulkActionsToolbar table={table} entityName={t('redemption code')}>
      <CopyButton
        value={contentToCopy}
        variant='outline'
        size='icon'
        tooltip={t('Copy selected codes')}
        successTooltip={t('Codes copied!')}
        aria-label={t('Copy selected codes')}
      />
      <Button
        variant='outline'
        size='sm'
        onClick={handleBulkDelete}
        disabled={isDeleting}
      >
        <Trash2 className='size-4' />
        {t('Delete selected')}
      </Button>
    </BulkActionsToolbar>
  )
}
