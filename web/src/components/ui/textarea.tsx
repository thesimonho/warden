import * as React from 'react'

import { cn } from '@/lib/utils'

function Textarea({ className, ...props }: React.ComponentProps<'textarea'>) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        // Layout & sizing
        'flex field-sizing-content min-h-16 w-full rounded border px-3 py-2',
        // Appearance
        'border-input bg-content-2 shadow-xs outline-none',
        // Typography
        'text-base md:text-sm',
        'placeholder:text-muted-foreground',
        'selection:bg-secondary selection:text-secondary-foreground',
        // Transitions
        'transition-[color,box-shadow]',
        // Focus
        'focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]',
        // Validation
        'aria-invalid:border-error aria-invalid:ring-error/20',
        'dark:bg-content-2 dark:aria-invalid:ring-error/40',
        // Disabled
        'disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      {...props}
    />
  )
}

export { Textarea }
