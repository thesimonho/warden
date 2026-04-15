import { Check, ChevronLeft, ChevronRight, FileUp, Loader2 } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'

import type { FormStep, StepBadge } from './project-config-form-types'
import { FORM_STEP_LABELS, FORM_STEPS } from './project-config-form-types'

/** Props for the StepTabs component. */
interface StepTabsProps {
  /** Currently active step. */
  currentStep: FormStep
  /** Callback when user clicks a different tab. */
  onStepChange: (step: FormStep) => void
  /** Summary text shown beneath each tab label. */
  summaries: Record<FormStep, string>
  /** Badge indicator for each tab. */
  badges: Record<FormStep, StepBadge>
}

/**
 * Horizontal step navigation bar for the project config form.
 *
 * Built on Radix Tabs for keyboard navigation and ARIA roles.
 * Each tab shows its label, a badge indicator (asterisk for required
 * fields, checkmark for configured), and a summary line describing
 * the current state of that step's fields.
 */
export function StepTabs({ currentStep, onStepChange, summaries, badges }: StepTabsProps) {
  return (
    <Tabs value={currentStep} onValueChange={(val) => onStepChange(val as FormStep)}>
      <TabsList className="h-auto w-full bg-transparent p-0">
        {FORM_STEPS.map((step) => {
          const badge = badges[step]
          return (
            <TabsTrigger
              key={step}
              value={step}
              className={cn(
                // Layout — override default trigger height for two-line content
                'flex h-auto flex-1 flex-col items-center gap-0.5 rounded-md px-3 py-2',
                // Active state — override default bg with primary tint
                'data-[state=active]:bg-secondary/40! data-[state=active]:text-foreground',
                'data-[state=active]:shadow-inner!',
              )}
            >
              <span className="flex items-center gap-1 text-sm font-medium">
                {FORM_STEP_LABELS[step]}
                {badge === 'required' && <span className="text-error text-xs">*</span>}
                {badge === 'configured' && (
                  <Check className="text-success h-3 w-3" strokeWidth={3} />
                )}
              </span>
              <span className="text-foreground/40 max-w-full truncate text-xs font-normal italic">
                {summaries[step]}
              </span>
            </TabsTrigger>
          )
        })}
      </TabsList>
    </Tabs>
  )
}

/** Props for the StepFooter component. */
interface StepFooterProps {
  /** Currently active step. */
  currentStep: FormStep
  /** Navigate to the previous step. */
  onBack: () => void
  /** Navigate to the next step. */
  onNext: () => void
  /** Submit the form. */
  onSubmit: () => void
  /** Whether the form passes validation. */
  isValid: boolean
  /** Whether a submission is in progress. */
  isSubmitting: boolean
  /** Whether the form is in create or edit mode. */
  mode: 'create' | 'edit'
  /** Triggers the template import file picker (create mode only). */
  onImport?: () => void
  /** Error message to display above the buttons. */
  error?: string | null
}

/**
 * Sticky footer bar for the project config form.
 *
 * Left side: Back/Next step buttons.
 * Right side: Import button (create mode only) and Create/Save button.
 * Error text renders above the button row when present.
 */
export function StepFooter({
  currentStep,
  onBack,
  onNext,
  onSubmit,
  isValid,
  isSubmitting,
  mode,
  onImport,
  error,
}: StepFooterProps) {
  const isFirst = currentStep === FORM_STEPS[0]
  const isLast = currentStep === FORM_STEPS[FORM_STEPS.length - 1]
  const isEditMode = mode === 'edit'

  return (
    <div className="space-y-2 border-t pt-3">
      {error && <p className="text-error text-sm">{error}</p>}

      <div className="flex items-center justify-between">
        {/* Left side: step navigation */}
        <div className="flex gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={onBack}
            disabled={isFirst || isSubmitting}
            icon={ChevronLeft}
          >
            Back
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={onNext}
            disabled={isLast || isSubmitting}
          >
            Next
            <ChevronRight className="h-4 w-4" />
          </Button>
        </div>

        {/* Right side: import + submit */}
        <div className="flex gap-2">
          {mode === 'create' && onImport && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onImport}
              disabled={isSubmitting}
              icon={FileUp}
            >
              Import
            </Button>
          )}
          <Button size="sm" onClick={onSubmit} disabled={isSubmitting || !isValid}>
            {isSubmitting ? (
              <>
                <Loader2 className="animate-spin" />
                {isEditMode ? 'Saving...' : 'Creating...'}
              </>
            ) : isEditMode ? (
              'Save'
            ) : (
              'Create'
            )}
          </Button>
        </div>
      </div>
    </div>
  )
}
