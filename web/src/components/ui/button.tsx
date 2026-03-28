import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { Slot } from 'radix-ui'

import { cn } from '@/lib/utils'

const buttonVariants = cva(
  "cursor-pointer inline-flex shrink-0 items-center justify-center gap-2 rounded font-medium whitespace-nowrap transition-all outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-error aria-invalid:ring-error/20 dark:aria-invalid:ring-error/40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground hover:bg-primary/90',
        error:
          'bg-error text-error-foreground hover:bg-error/90 focus-visible:ring-error/20 dark:bg-error/60 dark:focus-visible:ring-error/40',
        outline:
          'border bg-background shadow-xs hover:bg-accent hover:text-accent-foreground dark:border-input dark:bg-input/30 dark:hover:bg-input/50',
        secondary: 'bg-secondary text-secondary-foreground hover:bg-secondary/80',
        ghost: 'text-muted-foreground hover:bg-accent dark:hover:bg-accent/50',
        link: 'text-primary underline-offset-4 hover:underline',
        success:
          'bg-success text-success-foreground hover:bg-success/90 focus-visible:ring-success/20 dark:bg-success/60 dark:focus-visible:ring-success/40',
        warning:
          'bg-warning text-warning-foreground hover:bg-warning/90 focus-visible:ring-warning/20 dark:bg-warning/60 dark:focus-visible:ring-warning/40',
      },
      color: {
        primary: 'text-primary',
        secondary: 'text-secondary',
        accent: 'text-accent',
        error: 'text-error',
        success: 'text-success',
        warning: 'text-warning',
        muted: 'text-muted-foreground',
      },
      size: {
        default: 'h-9 px-4 py-2 has-[>svg]:px-3',
        xs: "h-6 gap-1 rounded px-2 text-sm has-[>svg]:px-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: 'h-8 gap-1.5 rounded px-3 has-[>svg]:px-2.5',
        lg: 'h-10 rounded px-6 has-[>svg]:px-4',
        icon: 'size-9',
        'icon-xs': "size-6 rounded [&_svg:not([class*='size-'])]:size-3",
        'icon-sm': 'size-8',
        'icon-lg': 'size-10',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)

/**
 * Button component with optional prepend icon slot.
 *
 * The `icon` prop accepts a component type (e.g. `Plus`, not `<Plus />`).
 * The icon is rendered before children, spaced via flex gap, and auto-sized
 * by the button's size variant. Icon dimensions are not overridable — the
 * button controls them.
 *
 * Set `loading` to replace the icon with a spinner animation.
 *
 * @param props.icon - Icon component rendered before children (e.g. `Plus`).
 * @param props.loading - When true, adds a spin animation to the icon.
 * @param props.asChild - Renders as a Slot, passing props to the child element.
 */
function Button({
  className,
  variant = 'default',
  size = 'default',
  color,
  asChild = false,
  icon: Icon,
  loading = false,
  children,
  ...props
}: React.ComponentProps<'button'> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
    icon?: React.ElementType
    loading?: boolean
  }) {
  const Comp = asChild ? Slot.Root : 'button'

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, color, className }))}
      {...props}
    >
      {asChild ? (
        children
      ) : (
        <>
          {Icon && <Icon className={loading ? 'animate-spin' : undefined} />}
          {children}
        </>
      )}
    </Comp>
  )
}

export { Button, buttonVariants }
