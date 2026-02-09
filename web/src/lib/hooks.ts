"use client";

import { useState, useCallback } from "react";

/**
 * Hook for copy-to-clipboard with "Copied!" feedback
 */
export function useCopyToClipboard(resetMs = 2000) {
  const [copied, setCopied] = useState(false);

  const copy = useCallback(
    async (text: string) => {
      try {
        await navigator.clipboard.writeText(text);
        setCopied(true);
        setTimeout(() => setCopied(false), resetMs);
      } catch (err) {
        console.error("Failed to copy:", err);
      }
    },
    [resetMs]
  );

  return { copied, copy };
}
