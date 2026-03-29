import * as React from 'react'
import { Switch as SwitchPrimitive } from 'radix-ui'

import { cn } from '@/lib/utils'

function Switch({
  className,
  size = 'default',
  ...props
}: React.ComponentProps<typeof SwitchPrimitive.Root> & {
  size?: 'sm' | 'default'
}) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      data-size={size}
      className={cn(
        // Layout
        'peer group/switch inline-flex shrink-0 items-center',
        'rounded-full border border-transparent',
        // Size variants
        'data-[size=default]:h-[1.15rem] data-[size=default]:w-8',
        'data-[size=sm]:h-3.5 data-[size=sm]:w-6',
        // Appearance
        'shadow-xs transition-all outline-none',
        // Checked/unchecked state
        'data-[state=checked]:bg-primary',
        'data-[state=unchecked]:bg-input dark:data-[state=unchecked]:bg-input/80',
        // Focus
        'focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]',
        // Disabled
        'disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        data-slot="switch-thumb"
        className={cn(
          // Layout
          'pointer-events-none block rounded-full ring-0',
          // Size variants
          'group-data-[size=default]/switch:size-4',
          'group-data-[size=sm]/switch:size-3',
          // Appearance
          'bg-background transition-transform',
          // Checked/unchecked state
          'data-[state=checked]:translate-x-[calc(100%-2px)]',
          'data-[state=unchecked]:translate-x-0',
          // Dark mode
          'dark:data-[state=checked]:bg-primary-foreground',
          'dark:data-[state=unchecked]:bg-foreground',
        )}
      />
    </SwitchPrimitive.Root>
  )
}

export { Switch }
