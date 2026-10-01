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
import { RotateCcw, Save } from 'lucide-react'
import {
  createContext,
  useContext,
  type ComponentProps,
  type ReactNode,
  type RefObject,
} from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

type SettingsPageContextValue = {
  actionsContainer: HTMLDivElement | null
  titleStatusContainer: HTMLSpanElement | null
  suppressSectionHeader: boolean
  readOnly: boolean
}

const SettingsPageContext = createContext<SettingsPageContextValue>({
  actionsContainer: null,
  titleStatusContainer: null,
  suppressSectionHeader: false,
  readOnly: false,
})

type SettingsPageProviderProps = {
  actionsContainer: HTMLDivElement | null
  titleStatusContainer?: HTMLSpanElement | null
  children: ReactNode
  suppressSectionHeader?: boolean
  /** Disables every control rendered through the page actions portal. */
  readOnly?: boolean
}

export function SettingsPageProvider(props: SettingsPageProviderProps) {
  return (
    <SettingsPageContext.Provider
      value={{
        actionsContainer: props.actionsContainer,
        titleStatusContainer: props.titleStatusContainer ?? null,
        suppressSectionHeader: props.suppressSectionHeader ?? true,
        readOnly: props.readOnly ?? false,
      }}
    >
      {props.children}
    </SettingsPageContext.Provider>
  )
}

export function useSuppressSettingsSectionHeader() {
  return useContext(SettingsPageContext).suppressSectionHeader
}

type SettingsPageTitleStatusPortalProps = {
  children: ReactNode
}

export function SettingsPageTitleStatusPortal(
  props: SettingsPageTitleStatusPortalProps
) {
  const { titleStatusContainer } = useContext(SettingsPageContext)

  if (!titleStatusContainer) return null

  return createPortal(props.children, titleStatusContainer)
}

type SettingsReadOnlyFieldsetProps = {
  readOnly: boolean
  className?: string
  children: ReactNode
}

/**
 * Groups settings controls so read-only mode disables them all at once.
 *
 * A disabled `<fieldset>` natively disables inputs, textareas and buttons
 * (including Select, Tabs and Accordion triggers). Base UI switches, checkboxes
 * and radios render a `<span>` over a hidden input, so they are additionally
 * made inert to the pointer and dimmed like other disabled controls.
 */
export function SettingsReadOnlyFieldset(props: SettingsReadOnlyFieldsetProps) {
  return (
    <fieldset
      disabled={props.readOnly}
      className={cn(
        'min-w-0',
        '[&:disabled_[data-slot=switch]]:pointer-events-none [&:disabled_[data-slot=switch]]:opacity-50',
        '[&:disabled_[data-slot=checkbox]]:pointer-events-none [&:disabled_[data-slot=checkbox]]:opacity-50',
        '[&:disabled_[data-slot=radio-group-item]]:pointer-events-none [&:disabled_[data-slot=radio-group-item]]:opacity-50',
        props.className
      )}
    >
      {props.children}
    </fieldset>
  )
}

type SettingsPageActionsPortalProps = {
  children: ReactNode
}

export function SettingsPageActionsPortal(
  props: SettingsPageActionsPortalProps
) {
  const { actionsContainer, readOnly } = useContext(SettingsPageContext)

  if (!actionsContainer) return null

  // Portaled actions render outside the page content, so they need their own
  // disabled fieldset to honour read-only mode.
  return createPortal(
    <SettingsReadOnlyFieldset
      readOnly={readOnly}
      className='flex flex-wrap items-center justify-end gap-2'
    >
      {props.children}
    </SettingsReadOnlyFieldset>,
    actionsContainer
  )
}

type SettingsPageFormActionsProps = {
  onSave: () => void
  onReset?: () => void
  isSaving?: boolean
  isSaveDisabled?: boolean
  isResetDisabled?: boolean
  saveLabel?: string
  savingLabel?: string
  resetLabel?: string
  resetVariant?: ComponentProps<typeof Button>['variant']
  saveButtonRef?: RefObject<HTMLButtonElement | null>
}

export function SettingsPageFormActions(props: SettingsPageFormActionsProps) {
  const { t } = useTranslation()
  const saveLabel = props.isSaving
    ? (props.savingLabel ?? 'Saving...')
    : (props.saveLabel ?? 'Save Changes')

  return (
    <SettingsPageActionsPortal>
      {props.onReset && (
        <Button
          type='button'
          size='sm'
          variant={props.resetVariant ?? 'outline'}
          onClick={props.onReset}
          disabled={props.isResetDisabled || props.isSaving}
        >
          <RotateCcw data-icon='inline-start' />
          <span>{t(props.resetLabel ?? 'Reset')}</span>
        </Button>
      )}
      <Button
        ref={props.saveButtonRef}
        type='button'
        size='sm'
        onClick={props.onSave}
        disabled={props.isSaving || props.isSaveDisabled}
      >
        <Save data-icon='inline-start' />
        <span>{t(saveLabel)}</span>
      </Button>
    </SettingsPageActionsPortal>
  )
}
