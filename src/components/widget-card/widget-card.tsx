import type { ReactNode } from "react";
import "./widget-card.css";

interface WidgetCardProps {
  readonly title: string;
  readonly children: ReactNode;
  readonly actionLabel?: string;
  readonly onAction?: () => void;
}

export default function WidgetCard({
  title,
  children,
  actionLabel,
  onAction,
}: WidgetCardProps) {
  return (
    <div className="widget-card">
      <h3 className="widget-title">{title}</h3>

      <div className="widget-content">{children}</div>

      {actionLabel && (
        <button className="upgrade-btn" onClick={onAction}>
          {actionLabel}
        </button>
      )}
    </div>
  );
}
