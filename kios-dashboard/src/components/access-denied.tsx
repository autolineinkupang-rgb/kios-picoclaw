import Link from "next/link";
import { ShieldX } from "lucide-react";
import { Card } from "./ui/card";
import { Button } from "./ui/button";

export function AccessDenied({
  message = "Halaman ini khusus untuk pemilik (owner) kios.",
}: {
  message?: string;
}) {
  return (
    <Card className="mx-auto max-w-md p-8 text-center">
      <div className="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-warning/15 text-warning">
        <ShieldX className="size-6" />
      </div>
      <h2 className="text-lg font-semibold">Akses terbatas</h2>
      <p className="mt-2 text-sm text-muted-foreground">{message}</p>
      <Link href="/dashboard" className="mt-5 inline-block">
        <Button variant="outline" size="sm">
          Kembali ke Dashboard
        </Button>
      </Link>
    </Card>
  );
}
