import * as React from 'react'

import { cn } from '@/lib/utils'

function Input({ className, type, ...props }: React.ComponentProps<'input'>) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        'border-input selection:bg-primary selection:text-primary-foreground file:text-foreground placeholder:text-muted-foreground disabled:bg-muted/50 dark:bg-content-2 bg-content-2 file:bg-content-2 h-9 w-full min-w-0 rounded border px-3 py-1 text-base shadow-xs transition-[color,box-shadow] outline-none file:inline-flex file:h-7 file:border-0 file:text-sm file:font-medium disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 md:text-sm',
        'focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]',
        'aria-invalid:border-error aria-invalid:ring-error/20 dark:aria-invalid:ring-error/40',
        className,
      )}
      {...props}
    />
  )
}

export { Input }
