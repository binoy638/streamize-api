import type { ButtonHTMLAttributes, CSSProperties, InputHTMLAttributes, ReactNode, SelectHTMLAttributes } from "react";
import { X } from "lucide-react";

import logoUrl from "../../assets/logo.png";

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
          <img className="brand-mark" src={logoUrl} alt="" />
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

export function Skeleton({
  className,
  style,
}: {
  className?: string;
  style?: CSSProperties;
}) {
  return <span className={cx("skeleton", className)} style={style} aria-hidden="true" />;
}

export function StatSkeletonGrid({ count = 4 }: { count?: number }) {
  return (
    <div className="stats-row" aria-hidden="true">
      {Array.from({ length: count }).map((_, index) => (
        <div className="stat-card stat-card-skeleton" key={index}>
          <Skeleton className="skeleton-label" />
          <Skeleton className="skeleton-value" />
          <Skeleton className="skeleton-line" style={{ width: "58%" }} />
        </div>
      ))}
    </div>
  );
}

export function TableSkeletonRows({
  rows = 5,
  columns,
}: {
  rows?: number;
  columns: number;
}) {
  return (
    <>
      {Array.from({ length: rows }).map((_, rowIndex) => (
        <tr className="skeleton-row" key={rowIndex} aria-hidden="true">
          {Array.from({ length: columns }).map((__, columnIndex) => (
            <td key={columnIndex}>
              <Skeleton
                className="skeleton-line"
                style={{ width: `${columnIndex === 0 ? 82 : 48 + ((rowIndex + columnIndex) % 4) * 10}%` }}
              />
              {columnIndex === 0 ? <Skeleton className="skeleton-line skeleton-subline" style={{ width: "54%" }} /> : null}
            </td>
          ))}
        </tr>
      ))}
    </>
  );
}

export function MediaGridSkeleton({ count = 6 }: { count?: number }) {
  return (
    <div className="media-grid" aria-hidden="true">
      {Array.from({ length: count }).map((_, index) => (
        <article className="media-card media-card-skeleton" key={index}>
          <Skeleton className="poster-skeleton" />
          <div className="media-body">
            <Skeleton className="skeleton-line" style={{ width: "72%" }} />
            <Skeleton className="skeleton-line" style={{ width: "88%" }} />
            <Skeleton className="skeleton-line" style={{ width: "100%" }} />
            <div className="media-actions">
              <Skeleton className="skeleton-button flex-1" />
              <Skeleton className="skeleton-icon" />
              <Skeleton className="skeleton-icon" />
            </div>
          </div>
        </article>
      ))}
    </div>
  );
}

export function DetailSkeleton() {
  return (
    <section className="content" aria-hidden="true">
      <div className="detail-hero">
        <Skeleton className="detail-poster" />
        <div className="detail-copy">
          <Skeleton className="skeleton-label" style={{ width: 160 }} />
          <Skeleton className="skeleton-heading" />
          <Skeleton className="skeleton-line" style={{ width: "92%" }} />
          <Skeleton className="skeleton-line" style={{ width: "70%" }} />
          <div className="component-row">
            <Skeleton className="skeleton-pill" />
            <Skeleton className="skeleton-pill" />
            <Skeleton className="skeleton-pill" />
          </div>
          <Skeleton className="skeleton-progress" />
        </div>
      </div>
      <StatSkeletonGrid />
      <div className="panel">
        <div className="skeleton-grid">
          <Skeleton />
          <Skeleton />
          <Skeleton />
        </div>
      </div>
    </section>
  );
}

export function PlayerSkeleton() {
  return (
    <section className="content player-content" aria-hidden="true">
      <div className="player-layout">
        <Skeleton className="player-frame-skeleton" />
        <aside className="panel player-side">
          <div className="panel-header">
            <div className="w-full">
              <Skeleton className="skeleton-line" style={{ width: "34%" }} />
              <Skeleton className="skeleton-line skeleton-subline" style={{ width: "56%" }} />
            </div>
          </div>
          <div className="file-list">
            <Skeleton className="file-row-skeleton" />
            <Skeleton className="file-row-skeleton" />
            <Skeleton className="file-row-skeleton" />
          </div>
        </aside>
      </div>
      <div className="player-meta">
        {Array.from({ length: 6 }).map((_, index) => (
          <div className="player-meta-item" key={index}>
            <Skeleton className="skeleton-label" />
            <Skeleton className="skeleton-line" style={{ width: 92 }} />
          </div>
        ))}
      </div>
    </section>
  );
}
