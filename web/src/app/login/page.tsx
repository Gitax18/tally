import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { LoginForm } from "@/components/login-form";
import { Logo } from "@/components/logo";
import { isSignedIn } from "@/lib/auth";

export const metadata: Metadata = { title: "Sign in" };

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ next?: string | string[] }>;
}) {
  if (await isSignedIn()) redirect("/");
  const { next } = await searchParams;
  return (
    <main className="grid min-h-screen place-items-center px-4">
      <div className="flex w-full max-w-sm flex-col gap-6">
        <div className="flex justify-center">
          <Logo />
        </div>
        <LoginForm next={typeof next === "string" ? next : "/"} />
      </div>
    </main>
  );
}
