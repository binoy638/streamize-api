import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode, SelectHTMLAttributes } from "react";
import { X } from "lucide-react";

export function cx(...classes: Array<string | false | null | undefined>) {
  return classes.filter(Boolean).join(" ");
}

export function Button({
  variant = "ghost",
  className,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "ghost" | "danger" }) {
  return <button className={cx("btn", `btn-${variant}`, className)} {...props} />;
}

export function Badge({
  children,
  tone = "default",
}: {
  children: ReactNode;
  tone?: string;
}) {
  return <span className={cx("badge", tone)}>{children}</span>;
}

export function Progress({ value }: { value: number }) {
  const bounded = Math.max(0, Math.min(100, value));
  return (
    <div className="progress" aria-label={`${bounded}% complete`}>
      <span style={{ width: `${bounded}%` }} />
    </div>
  );
}

export function Field({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="field">
      <label>{label}</label>
      {children}
    </div>
  );
}

export function Input(props: InputHTMLAttributes<HTMLInputElement>) {
  return <input className="input" {...props} />;
}

export function Select(props: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select className="select" {...props} />;
}

export function Textarea(props: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className="textarea" {...props} />;
}

export function Modal({
  title,
  open,
  onClose,
  children,
  footer,
  label,
}: {
  title: string;
  open: boolean;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
  label?: string;
}) {
  if (!open) {
    return null;
  }

  return (
    <div className="modal-backdrop is-open" role="presentation" onMouseDown={onClose}>
      <section
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-label={label || title}
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="modal-header">
          <strong>{title}</strong>
          <button className="icon-btn" type="button" onClick={onClose} aria-label="Close">
            <X size={16} />
          </button>
        </div>
        <div className="modal-body form-grid">{children}</div>
        {footer ? <div className="modal-footer">{footer}</div> : null}
      </section>
    </div>
  );
}

export function StatCard({
  label,
  value,
  detail,
}: {
  label: string;
  value: string;
  detail: string;
}) {
  return (
    <div className="stat-card">
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{detail}</small>
    </div>
  );
}

export function EmptyState({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <div className="empty-state">
      <strong>{title}</strong>
      <p className="muted">{children}</p>
    </div>
  );
}

export function LoadingScreen({ label = "Loading" }: { label?: string }) {
  return (
    <main className="auth-page">
      <div className="auth-card">
        <div className="brand" style={{ padding: 0, border: 0 }}>
          <div className="brand-mark">S</div>
          <div className="brand-copy">
            <strong>Streamize</strong>
            <span>{label}</span>
          </div>
        </div>
        <div className="skeleton-grid">
          <div className="skeleton" />
          <div className="skeleton" />
          <div className="skeleton" />
        </div>
      </div>
    </main>
  );
}
