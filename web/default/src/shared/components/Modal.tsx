import {
  useEffect,
  useId,
  useRef,
  type PropsWithChildren,
  type RefObject,
} from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";
import styles from "./Modal.module.css";

interface ModalProps extends PropsWithChildren {
  title: string;
  onClose: () => void;
  variant?: "dialog" | "drawer" | "wide";
  side?: "left" | "right";
  descriptionId?: string;
  closeDisabled?: boolean;
  initialFocus?: RefObject<HTMLElement | null>;
}

let modalCount = 0;
let previousOverflow = "";

export function Modal({
  title,
  onClose,
  variant = "dialog",
  side = "right",
  descriptionId,
  closeDisabled = false,
  initialFocus,
  children,
}: ModalProps) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  const titleId = useId();

  useEffect(() => {
    const dialog = dialogRef.current!;
    const trigger =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    if (modalCount === 0) {
      previousOverflow = document.body.style.overflow;
      document.body.style.overflow = "hidden";
    }
    modalCount += 1;
    dialog.showModal();
    (initialFocus?.current ?? titleRef.current)?.focus();
    return () => {
      dialog.close();
      modalCount -= 1;
      if (modalCount === 0) document.body.style.overflow = previousOverflow;
      if (trigger?.isConnected) trigger.focus();
    };
  }, [initialFocus]);

  return createPortal(
    <dialog
      ref={dialogRef}
      className={`${styles.modal} ${styles[variant]} ${variant === "drawer" && side === "left" ? styles.left : ""}`}
      aria-labelledby={titleId}
      aria-describedby={descriptionId}
      onCancel={(event) => {
        event.preventDefault();
        if (!closeDisabled) closeRef.current();
      }}
    >
      <header className={styles.header}>
        <h2 id={titleId} tabIndex={-1} ref={titleRef} className={styles.title}>
          {title}
        </h2>
        <button
          type="button"
          className={styles.close}
          aria-label="关闭"
          disabled={closeDisabled}
          onClick={onClose}
        >
          <X size={18} aria-hidden="true" />
        </button>
      </header>
      <div className={styles.body}>{children}</div>
    </dialog>,
    document.body,
  );
}
