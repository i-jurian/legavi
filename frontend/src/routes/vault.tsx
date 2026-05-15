import { useMemo, useState, type FormEvent } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { logout } from "@/api/auth";
import { useCryptoSession } from "@/store/cryptoSession";
import { deriveAgeKeypair } from "@/lib/age-keypair";
import { decryptPreview, encryptEntry } from "@/lib/vault-codec";
import { useCreateEntry, useVaultEntries } from "@/hooks/useVault";
import { consumeLockReason, useUnlock } from "@/lib/session";
import type { VaultEntrySummary } from "@/api/vault";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

const LOCK_MESSAGES = {
  idle: "Locked after 5 minutes of inactivity.",
  hidden: "Locked after this tab was hidden.",
  expired: "Server session expired.",
};

export function VaultPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const session = useCryptoSession();
  const { data, isPending, error } = useVaultEntries();
  const [logoutBusy, setLogoutBusy] = useState(false);
  const [logoutError, setLogoutError] = useState<string | null>(null);
  const { unlock, busy: unlockBusy, error: unlockError } = useUnlock();
  const [lockMessage] = useState<string>(
    () => consumeLockReason(LOCK_MESSAGES) ?? "Locked after page refresh.",
  );

  const keys = useMemo(() => {
    if (!session.ageIdentity) return null;
    return deriveAgeKeypair(session.ageIdentity);
  }, [session.ageIdentity]);

  async function onLogout() {
    setLogoutError(null);
    setLogoutBusy(true);
    try {
      await logout();
      useCryptoSession.getState().lock();
      queryClient.removeQueries({ queryKey: ["me"] });
      await navigate({ to: "/login" });
    } catch (err) {
      setLogoutError(err instanceof Error ? err.message : "logout failed");
    } finally {
      setLogoutBusy(false);
    }
  }

  if (session.state === "LOCKED" || !keys) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-background p-4">
        <Alert variant="destructive" className="max-w-sm">
          <AlertDescription className="flex flex-col items-start gap-3">
            <span>{lockMessage}</span>
            <Button size="sm" disabled={unlockBusy} onClick={unlock}>
              {unlockBusy ? "Unlocking..." : "Unlock vault"}
            </Button>
            {unlockError && (
              <span className="text-xs">{unlockError}</span>
            )}
          </AlertDescription>
        </Alert>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen items-start justify-center bg-background p-4">
      <Card className="w-full max-w-2xl">
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>Vault</CardTitle>
          <div className="flex gap-2">
            <NewEntryDialog recipient={keys.recipient} />
            <Button
              size="sm"
              variant="outline"
              disabled={logoutBusy}
              onClick={onLogout}
            >
              {logoutBusy ? "..." : "Sign out"}
            </Button>
          </div>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          {logoutError && (
            <Alert variant="destructive">
              <AlertDescription>{logoutError}</AlertDescription>
            </Alert>
          )}
          {isPending && (
            <p className="text-sm text-muted-foreground">Loading...</p>
          )}
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error.message}</AlertDescription>
            </Alert>
          )}
          {data && data.entries.length === 0 && (
            <p className="text-sm text-muted-foreground">No entries yet.</p>
          )}
          {data &&
            data.entries.map((entry) => (
              <VaultRow
                key={entry.id}
                entry={entry}
                identity={keys.identity}
              />
            ))}
        </CardContent>
      </Card>
    </main>
  );
}

function VaultRow({
  entry,
  identity,
}: {
  entry: VaultEntrySummary;
  identity: string;
}) {
  const {
    data: preview,
    isPending,
    error,
  } = useQuery({
    queryKey: ["vault", "preview", entry.id, entry.updatedAt],
    queryFn: () => decryptPreview(entry.preview, identity),
  });

  let label: string;
  if (isPending) label = "decrypting...";
  else if (error) label = "decrypt failed";
  else label = preview.label;

  return (
    <div className="flex items-center justify-between border-b py-2 last:border-b-0">
      <div>
        <p className="text-sm font-medium">{label}</p>
        <p className="text-xs text-muted-foreground">{entry.createdAt}</p>
      </div>
      {entry.deletedAt && (
        <span className="text-xs text-destructive">deleted</span>
      )}
    </div>
  );
}

function NewEntryDialog({ recipient }: { recipient: string }) {
  const createMut = useCreateEntry();
  const [open, setOpen] = useState(false);
  const [label, setLabel] = useState("");
  const [files, setFiles] = useState<File[]>([]);
  const [error, setError] = useState<string | null>(null);

  function reset() {
    setLabel("");
    setFiles([]);
    setError(null);
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!label.trim() || files.length === 0) return;
    setError(null);
    try {
      const { preview, bundle } = await encryptEntry(
        { label: label.trim(), files },
        [recipient],
      );
      await createMut.mutateAsync({
        preview,
        bundle,
        sortOrder: 0,
      });
      setOpen(false);
      reset();
    } catch (err) {
      setError(err instanceof Error ? err.message : "create failed");
    }
  }

  const submitDisabled =
    createMut.isPending || !label.trim() || files.length === 0;

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) reset();
      }}
    >
      <DialogTrigger asChild>
        <Button size="sm">New entry</Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New entry</DialogTitle>
        </DialogHeader>
        <form onSubmit={onSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="label">Label</Label>
            <Input
              id="label"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder="Bank credentials"
              required
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="files">Files</Label>
            <Input
              id="files"
              type="file"
              multiple
              onChange={(e) => setFiles(Array.from(e.target.files ?? []))}
              required
            />
            {files.length > 0 && (
              <ul className="text-xs text-muted-foreground">
                {files.map((f) => (
                  <li key={f.name}>
                    {f.name} ({f.size} bytes)
                  </li>
                ))}
              </ul>
            )}
          </div>
          {error && (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline">
                Cancel
              </Button>
            </DialogClose>
            <Button type="submit" disabled={submitDisabled}>
              {createMut.isPending ? "Creating..." : "Create"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
