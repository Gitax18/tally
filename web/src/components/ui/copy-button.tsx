"use client";

import { Check, Copy } from "lucide-react";
import * as React from "react";
import { Button } from "./button";

export function CopyButton({ text, label = "Copy" }: { text: string; label?: string }) {
  const [copied, setCopied] = React.useState(false);
  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // Clipboard blocked: the text is still selectable.
    }
  }
  return (
    <Button variant="secondary" size="sm" onClick={copy} aria-live="polite">
      {copied ? <Check aria-hidden /> : <Copy aria-hidden />}
      {copied ? "Copied" : label}
    </Button>
  );
}

/** The tracking snippet in a code block with a copy button. */
export function Snippet({ code }: { code: string }) {
  return (
    <div className="border-border bg-surface-2 rounded-control flex flex-col gap-3 border p-3 sm:flex-row sm:items-center">
      <code className="min-w-0 flex-1 font-mono text-[13px] break-all select-all">{code}</code>
      <CopyButton text={code} />
    </div>
  );
}
