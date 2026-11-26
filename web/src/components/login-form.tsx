"use client";

import * as React from "react";
import { login, type FormState } from "@/app/actions";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, Input } from "@/components/ui/input";

export function LoginForm({ next }: { next: string }) {
  const [state, action, pending] = React.useActionState<FormState, FormData>(login, {});
  return (
    <Card>
      <CardHeader>
        <CardTitle>Sign in</CardTitle>
        <CardDescription>Enter the admin password set on the tally server.</CardDescription>
      </CardHeader>
      <CardContent>
        <form action={action} className="flex flex-col gap-4">
          <input type="hidden" name="next" value={next} />
          <Field label="Password" htmlFor="password" error={state.error}>
            <Input
              id="password"
              name="password"
              type="password"
              autoComplete="current-password"
              autoFocus
              required
              aria-invalid={state.error ? true : undefined}
            />
          </Field>
          <Button type="submit" busy={pending}>
            Sign in
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
