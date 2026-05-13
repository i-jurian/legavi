import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { startRegistration } from "@simplewebauthn/browser";
import { registerStart, registerVerify } from "@/api/auth";
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

const PLACEHOLDER_AGE_RECIPIENT =
  "age1placeholder0000000000000000000000000000000000000000000000000";

export function RegisterPage() {
  const navigate = useNavigate();
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [nickname, setNickname] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      const { publicKey } = await registerStart({ email, displayName });
      const response = await startRegistration({ optionsJSON: publicKey });
      await registerVerify({
        ageRecipient: PLACEHOLDER_AGE_RECIPIENT,
        nickname: nickname.trim(),
        response,
      });
      await navigate({ to: "/dashboard" });
    } catch (err) {
      if (err instanceof Error && err.name === "NotAllowedError") {
        setError("Cancelled. Try again.");
      } else {
        setError(err instanceof Error ? err.message : "registration failed");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Register</CardTitle>
          <CardDescription>
            Create your account with a device passkey.
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
            <div className="flex flex-col gap-2">
              <Label htmlFor="displayName">Display name</Label>
              <Input
                id="displayName"
                type="text"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                required
                autoComplete="name"
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="nickname">Device nickname</Label>
              <Input
                id="nickname"
                type="text"
                value={nickname}
                onChange={(e) => setNickname(e.target.value)}
                required
                placeholder="e.g. MacBook"
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
              {busy ? "Working..." : "Register passkey"}
            </Button>
          </CardFooter>
        </form>
      </Card>
    </main>
  );
}
