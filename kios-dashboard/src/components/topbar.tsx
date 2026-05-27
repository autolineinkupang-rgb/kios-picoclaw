import type { SessionUser } from "@/lib/types";
import { MobileMenu } from "./mobile-menu";
import { PageTitle } from "./page-title";
import { ThemeToggle } from "./theme-toggle";
import { UserMenu } from "./user-menu";

export function Topbar({ user }: { user: SessionUser }) {
  return (
    <header className="sticky top-0 z-30 flex h-16 items-center justify-between gap-3 border-b bg-background/95 px-4 backdrop-blur sm:px-6">
      <div className="flex items-center gap-3">
        <MobileMenu />
        <PageTitle />
      </div>
      <div className="flex items-center gap-2">
        <ThemeToggle />
        <UserMenu user={user} />
      </div>
    </header>
  );
}
