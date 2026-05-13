import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { startAuthentication } from "@simplewebauthn/browser";
import { loginStart, loginVerify } from "@/api/auth";
import { readPRFFirst } from "@/lib/webauthn-prf";
import { useCryptoSession } from "@/stores/cryptoSession";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Alert, AlertDescription } from "@/components/ui/alert";

export function LoginPage() {
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      const { publicKey } = await loginStart({ email });
      const response = await startAuthentication({ optionsJSON: publicKey });

      const prfBytes = readPRFFirst(response);
      if (!prfBytes) {
        throw new Error(
          "This passkey doesn't support PRF. Try a different authenticator.",
        );
      }

      await loginVerify({ response });
      useCryptoSession.getState().unlock(prfBytes);
      await navigate({ to: "/dashboard" });
    } catch (err) {
      if (err instanceof Error && err.name === "NotAllowedError") {
        setError("Cancelled. Try again.");
      } else {
        setError(err instanceof Error ? err.message : "login failed");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Sign in</CardTitle>
          <CardDescription>
            Use your registered passkey to continue.
          </CardDescription>
        </CardHeader>
        <form onSubmit={onSubmit}>
          <CardContent className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                autoComplete="email"
              />
            </div>
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
          </CardContent>
          <CardFooter>
            <Button type="submit" disabled={busy} className="w-full">
              {busy ? "Working..." : "Sign in with passkey"}
            </Button>
          </CardFooter>
        </form>
      </Card>
    </main>
  );
}
