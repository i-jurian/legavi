import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { logout } from "@/api/auth";
import { useCryptoSession } from "@/store/cryptoSession";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";

export function DashboardPage() {
  const navigate = useNavigate();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onLogout() {
    setError(null);
    setBusy(true);
    try {
      await logout();
      useCryptoSession.getState().lock();
      await navigate({ to: "/login" });
    } catch (err) {
      setError(err instanceof Error ? err.message : "logout failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Dashboard</CardTitle>
          <CardDescription>You are signed in.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
        </CardContent>
        <CardFooter>
          <Button
            type="button"
            variant="outline"
            disabled={busy}
            onClick={onLogout}
            className="w-full"
          >
            {busy ? "Working..." : "Sign out"}
          </Button>
        </CardFooter>
      </Card>
    </main>
  );
}
