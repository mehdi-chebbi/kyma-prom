import React, { useEffect, useRef, useState } from "react";

interface TooltipProps {
  text: string;
  position?:
    | "top"
    | "bottom"
    | "left"
    | "right"
    | "top-left"
    | "top-right"
    | "bottom-left"
    | "bottom-right";
  color?: string; 
  delay?: number;
  children: React.ReactNode;
}

const positionClasses: Record<string, string> = {
  top: "bottom-full left-1/2 -translate-x-1/2 mb-2",
  bottom: "top-full left-1/2 -translate-x-1/2 mt-2",
  left: "right-full top-1/2 -translate-y-1/2 mr-2",
  right: "left-full top-1/2 -translate-y-1/2 ml-2",
  "top-left": "bottom-full left-0 mb-2",
  "top-right": "bottom-full right-0 mb-2",
  "bottom-left": "top-full left-0 mt-2",
  "bottom-right": "top-full right-0 mt-2",
};

export const Tooltip: React.FC<TooltipProps> = ({
  text,
  position = "top",
  color = "var(--dark-purple-500)",
  delay = 150,
  children,
}) => {
  const [visible, setVisible] = useState(false);
  const [flipped, setFlipped] = useState(false);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const boxRef = useRef<HTMLDivElement | null>(null);

  const show = () => {
    timeoutRef.current = setTimeout(() => setVisible(true), delay);
  };

  const hide = () => {
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }
    setVisible(false);
    setFlipped(false);
  };

  useEffect(() => {
    if (!visible || !boxRef.current) return;

    const rect = boxRef.current.getBoundingClientRect();
    const tooLeft = rect.left < 8;
    const tooRight = rect.right > globalThis.innerWidth - 8;

    const nextFlipped = tooLeft || tooRight;
    if (nextFlipped !== flipped) {
      const rafId = globalThis.requestAnimationFrame(() => {
        setFlipped(nextFlipped);
      });
      return () => globalThis.cancelAnimationFrame(rafId);
    }
  }, [visible, position, text, flipped]);

  const arrowStyle = (): React.CSSProperties => {
    const size = 6; 
    const colorVal = color || "var(--dark-purple-500)";


    switch (position) {
      case "top":
      case "top-left":
      case "top-right":
        return {
          position: "absolute",
          left: "50%",
          transform: "translateX(-50%)",
          top: "100%",
          width: 0,
          height: 0,
          borderLeft: `${size}px solid transparent`,
          borderRight: `${size}px solid transparent`,
          borderTop: `${size}px solid ${colorVal}`,
        };

      case "bottom":
      case "bottom-left":
      case "bottom-right":
        return {
          position: "absolute",
          left: "50%",
          transform: "translateX(-50%)",
          bottom: "100%",
          width: 0,
          height: 0,
          borderLeft: `${size}px solid transparent`,
          borderRight: `${size}px solid transparent`,
          borderBottom: `${size}px solid ${colorVal}`, 
        };

      case "left":
        return {
          position: "absolute",
          right: "-6px",
          top: "50%",
          transform: "translateY(-50%)",
          width: 0,
          height: 0,
          borderTop: `${size}px solid transparent`,
          borderBottom: `${size}px solid transparent`,
          borderLeft: `${size}px solid ${colorVal}`,
        };

      case "right":
        return {
          position: "absolute",
          left: "-5.6px",
          top: "50%",
          transform: "translateY(-50%)",
          width: 0,
          height: 0,
          borderTop: `${size}px solid transparent`,
          borderBottom: `${size}px solid transparent`,
          borderRight: `${size}px solid ${colorVal}`, 
        };

      default:
        return {};
    }
  };

  const posClass = positionClasses[position] ?? positionClasses.top;
  const flippedClass =
    flipped && (position === "top" || position === "bottom")
      ? "left-auto right-0 translate-x-0"
      : "";

  return (
    <button
      type="button"
      className="relative inline-block"
      aria-haspopup="true"
      aria-expanded={visible}
      onMouseEnter={show}
      onMouseLeave={hide}
      onTouchStart={show}
      onTouchEnd={hide}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") show();
        if (e.key === "Escape") hide();
      }}
      style={{ ["--tooltip-color"]: color } as React.CSSProperties}
    >
      {children}

      <div
        ref={boxRef}
        style={{
          backgroundColor: color,
          borderColor: color,
        }}
        className={`absolute px-2 py-1 text-sm rounded-lg shadow-lg whitespace-nowrap z-50 transition-all duration-150 origin-center
          ${posClass} ${flippedClass}
          ${visible ? "opacity-100 scale-100 pointer-events-auto" : "opacity-0 scale-90 pointer-events-none"}
        `}
      >
        {text}

        <span
          aria-hidden
          style={arrowStyle()}
          className="tooltip-arrow"
        />
      </div>
    </button>
  );
};
