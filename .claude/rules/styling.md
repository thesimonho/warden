# Styling

## Tailwind class organization

Long class strings in UI components MUST be split into grouped, commented lines rather than written as a single blob. Use `cn()` with multiple string arguments, or `.join(' ')` inside `cva()` base strings.

### Grouping order

1. **Layout** — positioning, display, flex/grid, alignment, gap, sizing, padding, margin
2. **Appearance** — background, border, rounded, shadow, outline, opacity
3. **Typography** — text size, font weight, color, whitespace, placeholder/selection pseudo-elements
4. **Transitions** — transition, duration, animation
5. **Focus** — focus-visible / focus ring styles
6. **State variants** — hover, active, checked, open/closed, data-[] attributes
7. **Validation** — aria-invalid styles
8. **Disabled** — disabled styles
9. **Dark mode** — dark: overrides (group with the concern they modify when short)
10. **Child selectors** — `[&_svg]`, `[&>*]`, etc.

### Example

```tsx
// Bad — one giant blob
className={cn(
  'peer border-input focus-visible:border-ring focus-visible:ring-ring/50 aria-invalid:border-error size-4 shrink-0 rounded border shadow-xs transition-shadow outline-none focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-50',
  className,
)}

// Good — grouped and commented
className={cn(
  // Layout & sizing
  'peer size-4 shrink-0 rounded border',
  // Appearance
  'border-input shadow-xs outline-none',
  // Transitions
  'transition-shadow',
  // Focus
  'focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50',
  // Validation
  'aria-invalid:border-error aria-invalid:ring-error/20',
  // Disabled
  'disabled:cursor-not-allowed disabled:opacity-50',
  className,
)}
```

### For `cva()` base strings

Use an array with `.join(' ')` since `cva()` expects a single string:

```tsx
const variants = cva(
  [
    // Layout
    'inline-flex items-center justify-center gap-2 rounded',
    // Typography
    'font-medium whitespace-nowrap',
    // Focus
    'focus-visible:ring-[3px] focus-visible:ring-ring/50',
  ].join(' '),
  { variants: { ... } },
)
```

### When NOT to split

Short class strings (roughly under 80 characters) are fine as a single line:

```tsx
className={cn('flex items-center gap-2 text-sm', className)}
```
