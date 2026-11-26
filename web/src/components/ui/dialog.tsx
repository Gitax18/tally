"use client";

import { X } from "lucide-react";
import { Dialog as D } from "radix-ui";
import * as React from "react";
import { cn } from "@/lib/utils";

export const Dialog = D.Root;
export const DialogTrigger = D.Trigger;
export const DialogClose = D.Close;

export function DialogContent({
  title,
  description,
  className,
  children,
}: {
  title: string;
  description?: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <D.Portal>
      <D.Overlay className="animate-fade-in fixed inset-0 z-40 bg-black/40" />
      <D.Content
        className={cn(
          "bg-card text-card-foreground rounded-card border-border shadow-lift animate-fade-in fixed top-1/2 left-1/2 z-50 flex max-h-[85vh] w-[calc(100vw-2rem)] max-w-2xl -translate-x-1/2 -translate-y-1/2 flex-col border",
          className,
        )}
      >
        <div className="border-border flex items-start justify-between gap-4 border-b px-5 py-4">
          <div>
            <D.Title className="text-base font-semibold">{title}</D.Title>
            {description ? (
              <D.Description className="text-muted-foreground text-sm">{description}</D.Description>
            ) : (
              <D.Description className="sr-only">{title}</D.Description>
            )}
          </div>
          <D.Close
            className="text-muted-foreground hover:bg-surface-2 hover:text-foreground rounded-control grid size-8 place-items-center"
            aria-label="Close"
          >
            <X className="size-4" />
          </D.Close>
        </div>
        {children}
      </D.Content>
    </D.Portal>
  );
}
