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
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it } from 'vitest'

import { usageLogSchema } from '../../data/schema'
import { useCommonLogsColumns } from '../columns/common-logs-columns'

const log = usageLogSchema.parse({
  id: 1,
  user_id: 2,
  created_at: 1788840000,
  type: 2,
  content: '',
  model_name: 'requested-model',
  quota: 1000,
  other: JSON.stringify({
    is_model_mapped: true,
    upstream_model_name: 'mapped-model',
    response_model: {
      requested_model: 'requested-model',
      upstream_model: 'mapped-model',
      returned_model: 'unexpected-model',
    },
  }),
})

function ModelCell(props: { isAdmin: boolean }) {
  const table = useReactTable({
    data: [log],
    columns: useCommonLogsColumns(props.isAdmin, false),
    getCoreRowModel: getCoreRowModel(),
  })
  const cell = table
    .getRowModel()
    .rows[0].getAllCells()
    .find((item) => item.column.id === 'model_name')
  if (!cell) throw new Error('The log must have a model column')
  return flexRender(cell.column.columnDef.cell, cell.getContext())
}

it('opens the model routing popover from the model column for an admin', async () => {
  const user = userEvent.setup()
  render(<ModelCell isAdmin />)
  await user.click(
    screen.getByRole('button', {
      name: 'Model: requested-model, Response model: unexpected-model',
    })
  )
  expect(await screen.findByText('Upstream Model')).toBeVisible()
  expect(screen.getByText('mapped-model')).toBeVisible()
})

it('shows only the requested model in the model column for a regular user', () => {
  render(<ModelCell isAdmin={false} />)
  expect(screen.getByText('requested-model')).toBeVisible()
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
  expect(screen.queryByText(/unexpected-model/)).not.toBeInTheDocument()
})
