import { LogOut } from "lucide-react";
import { logout } from "@/app/actions";
import { Logo } from "@/components/logo";
import { Button } from "@/components/ui/button";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col">
      <header className="border-border bg-card/80 sticky top-0 z-30 border-b backdrop-blur">
        <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4 sm:px-6">
          <Logo />
          <form action={logout}>
            <Button type="submit" variant="ghost" size="sm">
              <LogOut aria-hidden />
              Sign out
            </Button>
          </form>
        </div>
      </header>
      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-6 sm:px-6 sm:py-8">{children}</main>
    </div>
  );
}
