import * as React from 'react'

import { cn } from '@/lib/utils'

function Input({ className, type, ...props }: React.ComponentProps<'input'>) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        // Layout & sizing
        'h-9 w-full min-w-0 rounded border px-3 py-1',
        // Appearance
        'border-input bg-content-2 shadow-xs outline-none',
        // Typography
        'text-base md:text-sm',
        'placeholder:text-muted-foreground',
        'selection:bg-primary selection:text-primary-foreground',
        // Transitions
        'transition-[color,box-shadow]',
        // File input
        'file:bg-content-2 file:inline-flex file:h-7 file:border-0',
        'file:text-foreground file:text-sm file:font-medium',
        // Focus
        'focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]',
        // Validation
        'aria-invalid:border-error aria-invalid:ring-error/20',
        'dark:aria-invalid:ring-error/40',
        // Disabled
        'disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50',
        'disabled:bg-muted/50',
        // Dark mode
        'dark:bg-content-2',
        className,
      )}
      {...props}
    />
  )
}

export { Input }
