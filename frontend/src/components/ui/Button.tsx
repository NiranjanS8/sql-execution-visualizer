import type { ButtonHTMLAttributes } from "react";

import { cn } from "../../lib/cn";

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "ghost";
  size?: "sm" | "md";
};

const base =
  "inline-flex items-center justify-center gap-1.5 rounded-sm font-medium transition-colors disabled:pointer-events-none disabled:opacity-50";

const variants = {
  primary: "bg-accent text-accent-ink hover:brightness-110",
  ghost: "border border-line text-ink-muted hover:bg-hover hover:text-ink",
};

const sizes = {
  sm: "h-7 px-2 text-xs",
  md: "h-8 px-3 text-sm",
};

export function Button({
  variant = "ghost",
  size = "sm",
  type = "button",
  className,
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      className={cn(base, variants[variant], sizes[size], className)}
      {...props}
    />
  );
}
