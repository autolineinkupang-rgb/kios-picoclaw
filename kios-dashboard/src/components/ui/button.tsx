import * as React from "react";
import { cn } from "@/lib/utils";

type Variant = "default" | "accent" | "outline" | "ghost" | "destructive";
type Size = "sm" | "md" | "icon";

const VARIANTS: Record<Variant, string> = {
  default:
    "bg-primary text-primary-foreground hover:bg-primary/90 focus-visible:ring-ring",
  accent:
    "bg-accent text-accent-foreground hover:bg-accent/90 focus-visible:ring-accent",
  outline:
    "border border-input bg-transparent hover:bg-muted focus-visible:ring-ring",
  ghost: "bg-transparent hover:bg-muted focus-visible:ring-ring",
  destructive:
    "bg-destructive text-destructive-foreground hover:bg-destructive/90 focus-visible:ring-destructive",
};

const SIZES: Record<Size, string> = {
  sm: "h-9 px-3 text-sm gap-1.5",
  md: "h-11 px-5 text-sm gap-2",
  icon: "size-11",
};

export interface ButtonProps extends React.ComponentProps<"button"> {
  variant?: Variant;
  size?: Size;
}

export function Button({
  className,
  variant = "default",
  size = "md",
  type = "button",
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      className={cn(
        "inline-flex cursor-pointer items-center justify-center rounded-lg font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:pointer-events-none disabled:opacity-50",
        VARIANTS[variant],
        SIZES[size],
        className,
      )}
      {...props}
    />
  );
}
