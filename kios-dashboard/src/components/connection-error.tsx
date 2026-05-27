import { DatabaseZap } from "lucide-react";
import { Card } from "./ui/card";

export function ConnectionError({ message }: { message?: string }) {
  return (
    <Card className="mx-auto max-w-xl p-8 text-center">
      <div className="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-destructive/15 text-destructive">
        <DatabaseZap className="size-6" />
      </div>
      <h2 className="text-lg font-semibold">Gagal terhubung ke data kios</h2>
      <p className="mt-2 text-sm text-muted-foreground">
        Pastikan <code className="font-mono">UPSTASH_REDIS_REST_URL</code> dan{" "}
        <code className="font-mono">UPSTASH_REDIS_REST_TOKEN</code> sudah di-set dan
        menunjuk ke database yang sama dengan bot Telegram.
      </p>
      {message && (
        <pre className="mt-4 overflow-auto rounded-lg bg-muted p-3 text-left text-xs text-muted-foreground">
          {message}
        </pre>
      )}
    </Card>
  );
}
