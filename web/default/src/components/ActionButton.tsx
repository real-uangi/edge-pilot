import { forwardRef, type ButtonHTMLAttributes } from "react";
import styles from "../styles/admin.module.css";

type ActionVariant = "primary" | "secondary" | "danger" | "ghost";

const variantClassMap: Record<ActionVariant, string> = {
  primary: styles.primaryButton,
  secondary: styles.secondaryButton,
  danger: styles.dangerButton,
  ghost: styles.ghostButton,
};

interface ActionButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children"> {
  label: string;
  pending?: boolean;
  pendingLabel?: string;
  variant?: ActionVariant;
}

export const ActionButton = forwardRef<HTMLButtonElement, ActionButtonProps>(function ActionButton(
  {
    label,
    pending = false,
    pendingLabel,
    variant = "secondary",
    disabled,
    type = "button",
    ...rest
  },
  ref,
) {
  return (
    <button
      className={variantClassMap[variant]}
      disabled={disabled || pending}
      ref={ref}
      type={type}
      {...rest}
    >
      {pending ? pendingLabel ?? `${label}中` : label}
    </button>
  );
});
