import { useEffect, useState } from "react";

export function useDelayedLoading(loading: boolean, delay = 250): boolean {
  const [show, setShow] = useState(false);

  useEffect(() => {
    let timer: number;

    if (loading) {
      timer = globalThis.setTimeout(() => setShow(true), delay);
    } else {
      setShow(false);
    }

    return () => clearTimeout(timer);
  }, [loading, delay]);

  return show;
}
