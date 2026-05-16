import {
  useMemo,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { logout } from "@/api/auth";
import { useCryptoSession } from "@/store/cryptoSession";
import { deriveAgeKeypair } from "@/lib/age-keypair";
import {
  decryptBundle,
  decryptPreview,
  encryptEntry,
} from "@/lib/vault-codec";
import {
  useCreateEntry,
  useUpdateEntry,
  useVaultEntries,
} from "@/hooks/useVault";
import { consumeLockReason, useUnlock } from "@/lib/session";
import { getEntry, type VaultEntrySummary } from "@/api/vault";
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

type EntryFormData = { label: string; files: File[] };

const EMPTY_FORM: EntryFormData = { label: "", files: [] };

type Keys = { identity: string; recipient: string };

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

  const keys = useMemo<Keys | null>(() => {
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
            <CreateEntryButton keys={keys} />
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
              <VaultRow key={entry.id} entry={entry} keys={keys} />
            ))}
        </CardContent>
      </Card>
    </main>
  );
}

function VaultRow({ entry, keys }: { entry: VaultEntrySummary; keys: Keys }) {
  const [editOpen, setEditOpen] = useState(false);
  const previewQuery = useQuery({
    queryKey: ["vault", "preview", entry.id, entry.updatedAt],
    queryFn: () => decryptPreview(entry.preview, keys.identity),
  });
  const editQuery = useQuery({
    queryKey: ["vault", "edit", entry.id, entry.updatedAt],
    queryFn: async (): Promise<EntryFormData> => {
      const full = await getEntry(entry.id);
      const [preview, files] = await Promise.all([
        decryptPreview(full.preview, keys.identity),
        decryptBundle(full.bundle, keys.identity),
      ]);
      return { label: preview.label, files };
    },
    enabled: editOpen,
  });
  const updateMut = useUpdateEntry();

  let label: string;
  if (previewQuery.isPending) label = "decrypting...";
  else if (previewQuery.error) label = "decrypt failed";
  else label = previewQuery.data.label;

  async function handleUpdate(data: EntryFormData) {
    const { preview, bundle } = await encryptEntry(
      { label: data.label, files: data.files },
      [keys.recipient],
    );
    await updateMut.mutateAsync({
      id: entry.id,
      body: { preview, bundle, sortOrder: entry.sortOrder },
    });
  }

  return (
    <div className="flex items-center justify-between border-b py-2 last:border-b-0">
      <div>
        <p className="text-sm font-medium">{label}</p>
        <p className="text-xs text-muted-foreground">{entry.createdAt}</p>
      </div>
      <div className="flex items-center gap-2">
        {entry.deletedAt && (
          <span className="text-xs text-destructive">deleted</span>
        )}
        <EntryFormDialog
          trigger={
            <Button size="sm" variant="outline">
              Edit
            </Button>
          }
          title="Edit entry"
          submitLabel="Save"
          open={editOpen}
          onOpenChange={setEditOpen}
          initial={editQuery.data ?? null}
          busy={updateMut.isPending}
          onSubmit={handleUpdate}
        />
      </div>
    </div>
  );
}

function CreateEntryButton({ keys }: { keys: Keys }) {
  const [open, setOpen] = useState(false);
  const createMut = useCreateEntry();

  async function handleCreate(data: EntryFormData) {
    const { preview, bundle } = await encryptEntry(
      { label: data.label, files: data.files },
      [keys.recipient],
    );
    await createMut.mutateAsync({ preview, bundle, sortOrder: 0 });
  }

  return (
    <EntryFormDialog
      trigger={<Button size="sm">New entry</Button>}
      title="New entry"
      submitLabel="Create"
      open={open}
      onOpenChange={setOpen}
      initial={open ? EMPTY_FORM : null}
      busy={createMut.isPending}
      onSubmit={handleCreate}
    />
  );
}

function EntryFormDialog({
  trigger,
  title,
  submitLabel,
  open,
  onOpenChange,
  initial,
  busy,
  onSubmit,
}: {
  trigger: ReactNode;
  title: string;
  submitLabel: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initial: EntryFormData | null;
  busy: boolean;
  onSubmit: (data: EntryFormData) => Promise<void>;
}) {
  const [label, setLabel] = useState("");
  const [files, setFiles] = useState<File[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [populated, setPopulated] = useState<EntryFormData | null>(null);

  if (open && initial && populated !== initial) {
    setPopulated(initial);
    setLabel(initial.label);
    setFiles(initial.files);
    setError(null);
  }
  if (!open && populated !== null) {
    setPopulated(null);
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!label.trim() || files.length === 0) return;
    setError(null);
    try {
      await onSubmit({ label: label.trim(), files });
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "save failed");
    }
  }

  function onPickFiles(e: React.ChangeEvent<HTMLInputElement>) {
    const picked = Array.from(e.target.files ?? []);
    if (picked.length === 0) return;
    setFiles((curr) => {
      const map = new Map(curr.map((f) => [f.name, f]));
      for (const p of picked) map.set(p.name, p);
      return Array.from(map.values());
    });
    e.target.value = "";
  }

  const submitDisabled = busy || !label.trim() || files.length === 0;
  const ready = open && initial;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        {!ready ? (
          <p className="text-sm text-muted-foreground">Decrypting...</p>
        ) : (
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
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
                onChange={onPickFiles}
              />
              {files.length > 0 && (
                <ul className="flex flex-col gap-1 text-xs text-muted-foreground">
                  {files.map((f) => (
                    <li
                      key={f.name}
                      className="flex items-center justify-between"
                    >
                      <span>
                        {f.name} ({f.size} bytes)
                      </span>
                      <button
                        type="button"
                        onClick={() =>
                          setFiles((curr) =>
                            curr.filter((x) => x.name !== f.name),
                          )
                        }
                        className="text-destructive hover:underline"
                      >
                        remove
                      </button>
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
                {busy ? "Saving..." : submitLabel}
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
