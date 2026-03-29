import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { Slot } from 'radix-ui'

import { cn } from '@/lib/utils'

const badgeVariants = cva(
  [
    // Layout & sizing
    'inline-flex w-fit shrink-0 items-center justify-center gap-1',
    'overflow-hidden rounded border border-transparent px-2 py-0.5',
    // Typography
    'text-sm font-medium whitespace-nowrap',
    // Transitions
    'transition-[color,box-shadow]',
    // Focus
    'focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50',
    // Validation
    'aria-invalid:border-error aria-invalid:ring-error/20',
    'dark:aria-invalid:ring-error/40',
    // Child SVG
    '[&>svg]:pointer-events-none [&>svg]:size-3',
  ].join(' '),
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground [a&]:hover:bg-primary/90',
        secondary: 'bg-secondary text-secondary-foreground [a&]:hover:bg-secondary/90',
        success:
          'bg-success text-success-foreground focus-visible:ring-success/20 dark:bg-success/60 dark:focus-visible:ring-success/40 [a&]:hover:bg-success/90',
        warning:
          'bg-warning text-warning-foreground focus-visible:ring-warning/20 dark:bg-warning/60 dark:focus-visible:ring-warning/40 [a&]:hover:bg-warning/90',
        error:
          'bg-error text-error-foreground focus-visible:ring-error/20 dark:bg-error/60 dark:focus-visible:ring-error/40 [a&]:hover:bg-error/90',
        outline:
          'border-border text-foreground [a&]:hover:bg-accent [a&]:hover:text-accent-foreground',
        ghost: '[a&]:hover:bg-accent [a&]:hover:text-accent-foreground',
        link: 'text-primary underline-offset-4 [a&]:hover:underline',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
)

function Badge({
  className,
  variant = 'default',
  asChild = false,
  ...props
}: React.ComponentProps<'span'> & VariantProps<typeof badgeVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot.Root : 'span'

  return (
    <Comp
      data-slot="badge"
      data-variant={variant}
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  )
}

export { Badge, badgeVariants }
