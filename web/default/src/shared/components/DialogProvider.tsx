import {
  createContext,
  type PropsWithChildren,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { ActionButton } from "./ActionButton";
import styles from "./DialogProvider.module.css";

interface BaseDialogOptions {
  title?: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  danger?: boolean;
  defaultValue?: string;
  placeholder?: string;
  inputLabel?: string;
}

export interface AlertDialogOptions extends BaseDialogOptions {}

export interface ConfirmDialogOptions extends BaseDialogOptions {}

export interface PromptDialogOptions extends BaseDialogOptions {}

type AlertInput = string | AlertDialogOptions;
type ConfirmInput = string | ConfirmDialogOptions;
type PromptInput = string | PromptDialogOptions;

interface DialogController {
  alert: (input: AlertInput) => Promise<void>;
  confirm: (input: ConfirmInput) => Promise<boolean>;
  prompt: (input: PromptInput) => Promise<string | null>;
}

type DialogRequest =
  | { kind: "alert"; options: AlertDialogOptions; resolve: () => void }
  | { kind: "confirm"; options: ConfirmDialogOptions; resolve: (value: boolean) => void }
  | { kind: "prompt"; options: PromptDialogOptions; resolve: (value: string | null) => void };

const dialogContext = createContext<DialogController | null>(null);

function normalizeAlertInput(input: AlertInput): AlertDialogOptions {
  if (typeof input === "string") {
    return { message: input };
  }
  return input;
}

function normalizeConfirmInput(input: ConfirmInput): ConfirmDialogOptions {
  if (typeof input === "string") {
    return { message: input };
  }
  return input;
}

function normalizePromptInput(input: PromptInput): PromptDialogOptions {
  if (typeof input === "string") {
    return { message: input };
  }
  return input;
}

export function DialogProvider({ children }: PropsWithChildren) {
  const queueRef = useRef<DialogRequest[]>([]);
  const confirmRef = useRef<HTMLButtonElement | null>(null);
  const promptRef = useRef<HTMLInputElement | null>(null);
  const [activeDialog, setActiveDialog] = useState<DialogRequest | null>(null);
  const [promptValue, setPromptValue] = useState("");
  const titleId = useId();
  const messageId = useId();

  const dequeue = useCallback(() => {
    setActiveDialog(queueRef.current.shift() ?? null);
  }, []);

  const enqueue = useCallback((dialog: DialogRequest) => {
    setActiveDialog((current) => {
      if (current) {
        queueRef.current.push(dialog);
        return current;
      }
      return dialog;
    });
  }, []);

  const controller = useMemo<DialogController>(
    () => ({
      alert: (input) =>
        new Promise<void>((resolve) => {
          enqueue({ kind: "alert", options: normalizeAlertInput(input), resolve });
        }),
      confirm: (input) =>
        new Promise<boolean>((resolve) => {
          enqueue({ kind: "confirm", options: normalizeConfirmInput(input), resolve });
        }),
      prompt: (input) =>
        new Promise<string | null>((resolve) => {
          enqueue({ kind: "prompt", options: normalizePromptInput(input), resolve });
        }),
    }),
    [enqueue],
  );

  useEffect(() => {
    if (!activeDialog) {
      return;
    }
    const originalOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = originalOverflow;
    };
  }, [activeDialog]);

  useEffect(() => {
    if (!activeDialog) {
      return;
    }
    if (activeDialog.kind === "prompt") {
      setPromptValue(activeDialog.options.defaultValue ?? "");
      requestAnimationFrame(() => promptRef.current?.focus());
      return;
    }
    setPromptValue("");
    requestAnimationFrame(() => confirmRef.current?.focus());
  }, [activeDialog]);

  const resolveAlert = () => {
    if (!activeDialog || activeDialog.kind !== "alert") {
      return;
    }
    activeDialog.resolve();
    dequeue();
  };

  const resolveConfirm = (result: boolean) => {
    if (!activeDialog || activeDialog.kind !== "confirm") {
      return;
    }
    activeDialog.resolve(result);
    dequeue();
  };

  const resolvePrompt = (result: string | null) => {
    if (!activeDialog || activeDialog.kind !== "prompt") {
      return;
    }
    activeDialog.resolve(result);
    dequeue();
  };

  let dialogElement: ReactNode = null;
  if (activeDialog) {
    const options = activeDialog.options;
    const title =
      options.title ?? (activeDialog.kind === "alert" ? "提示" : activeDialog.kind === "confirm" ? "请确认" : "请输入");

    dialogElement = createPortal(
      <div className={styles.backdrop}>
        <div aria-describedby={messageId} aria-labelledby={titleId} aria-modal="true" className={styles.dialog} role="dialog">
          <h2 className={styles.title} id={titleId}>
            {title}
          </h2>
          <p className={styles.message} id={messageId}>
            {options.message}
          </p>

          {activeDialog.kind === "prompt" ? (
            <label className={styles.promptField}>
              <span className={styles.promptLabel}>{options.inputLabel ?? "输入内容"}</span>
              <input
                className={styles.input}
                onChange={(event) => setPromptValue(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    resolvePrompt(promptValue);
                  }
                }}
                placeholder={options.placeholder}
                ref={promptRef}
                type="text"
                value={promptValue}
              />
            </label>
          ) : null}

          <div className={styles.actions}>
            {activeDialog.kind === "alert" ? null : (
              <ActionButton
                label={options.cancelText ?? "取消"}
                onClick={() => {
                  if (activeDialog.kind === "confirm") {
                    resolveConfirm(false);
                    return;
                  }
                  resolvePrompt(null);
                }}
                variant="secondary"
              />
            )}
            <ActionButton
              label={options.confirmText ?? (activeDialog.kind === "alert" ? "知道了" : "确认")}
              onClick={() => {
                if (activeDialog.kind === "alert") {
                  resolveAlert();
                  return;
                }
                if (activeDialog.kind === "confirm") {
                  resolveConfirm(true);
                  return;
                }
                resolvePrompt(promptValue);
              }}
              ref={activeDialog.kind === "prompt" ? null : confirmRef}
              variant={options.danger ? "danger" : "primary"}
            />
          </div>
        </div>
      </div>,
      document.body,
    );
  }

  return (
    <dialogContext.Provider value={controller}>
      {children}
      {dialogElement}
    </dialogContext.Provider>
  );
}

export function useDialog() {
  const dialog = useContext(dialogContext);
  if (!dialog) {
    throw new Error("useDialog must be used within DialogProvider");
  }
  return dialog;
}
