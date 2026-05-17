import {
  useMemo,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { GripVertical } from "lucide-react";
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
  useDeleteEntry,
  useReorderEntries,
  useRestoreEntry,
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

const SORT_ORDER_STEP = 100;

type EntryFormData = { label: string; files: File[] };

const EMPTY_FORM: EntryFormData = { label: "", files: [] };

const RESTORE_WINDOW_MS = 30 * 24 * 60 * 60 * 1000;

function isRestorable(deletedAt: string | null): boolean {
  if (!deletedAt) return false;
  return Date.now() - new Date(deletedAt).getTime() < RESTORE_WINDOW_MS;
}

type Keys = { identity: string; recipient: string };

export function VaultPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const session = useCryptoSession();
  const [showDeleted, setShowDeleted] = useState(false);
  const { data, isPending, error } = useVaultEntries({
    includeDeleted: showDeleted,
  });
  const [logoutBusy, setLogoutBusy] = useState(false);
  const [logoutError, setLogoutError] = useState<string | null>(null);
  const { unlock, busy: unlockBusy, error: unlockError } = useUnlock();
  const [lockMessage] = useState<string>(
    () => consumeLockReason(LOCK_MESSAGES) ?? "Locked after page refresh.",
  );
  const reorderMut = useReorderEntries();

  const keys = useMemo<Keys | null>(() => {
    if (!session.ageIdentity) return null;
    return deriveAgeKeypair(session.ageIdentity);
  }, [session.ageIdentity]);

  const activeEntries = useMemo(
    () => data?.entries.filter((e) => !e.deletedAt) ?? [],
    [data],
  );
  const deletedEntries = useMemo(
    () => data?.entries.filter((e) => e.deletedAt) ?? [],
    [data],
  );
  const nextSortOrder = useMemo(() => {
    if (activeEntries.length === 0) return SORT_ORDER_STEP;
    const max = Math.max(...activeEntries.map((e) => e.sortOrder));
    return max + SORT_ORDER_STEP;
  }, [activeEntries]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );

  function handleDragEnd(event: DragEndEvent) {
    const { active, over } = event;
    if (!over || active.id === over.id) return;

    const oldIndex = activeEntries.findIndex((e) => e.id === active.id);
    const newIndex = activeEntries.findIndex((e) => e.id === over.id);
    if (oldIndex < 0 || newIndex < 0) return;

    const reordered = arrayMove(activeEntries, oldIndex, newIndex);
    const orders = reordered.map((e, i) => ({
      id: e.id,
      sortOrder: (i + 1) * SORT_ORDER_STEP,
    }));
    reorderMut.mutate(orders);
  }

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
            {unlockError && <span className="text-xs">{unlockError}</span>}
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
            <Button
              size="sm"
              variant="outline"
              onClick={() => setShowDeleted((v) => !v)}
            >
              {showDeleted ? "Hide deleted" : "Show deleted"}
            </Button>
            <CreateEntryButton keys={keys} nextSortOrder={nextSortOrder} />
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
          {data && activeEntries.length === 0 && deletedEntries.length === 0 && (
            <p className="text-sm text-muted-foreground">No entries yet.</p>
          )}
          {reorderMut.error && (
            <Alert variant="destructive">
              <AlertDescription>{reorderMut.error.message}</AlertDescription>
            </Alert>
          )}
          <DndContext
            sensors={sensors}
            collisionDetection={closestCenter}
            onDragEnd={handleDragEnd}
          >
            <SortableContext
              items={activeEntries.map((e) => e.id)}
              strategy={verticalListSortingStrategy}
            >
              {activeEntries.map((entry) => (
                <SortableVaultRow key={entry.id} entry={entry} keys={keys} />
              ))}
            </SortableContext>
          </DndContext>
          {showDeleted &&
            deletedEntries.map((entry) => (
              <VaultRow key={entry.id} entry={entry} keys={keys} />
            ))}
        </CardContent>
      </Card>
    </main>
  );
}

function SortableVaultRow({
  entry,
  keys,
}: {
  entry: VaultEntrySummary;
  keys: Keys;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } =
    useSortable({ id: entry.id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };
  return (
    <div ref={setNodeRef} style={style}>
      <VaultRow
        entry={entry}
        keys={keys}
        dragHandle={
          <button
            type="button"
            {...attributes}
            {...listeners}
            className="cursor-grab touch-none text-muted-foreground hover:text-foreground"
            aria-label="Drag to reorder"
          >
            <GripVertical size={16} />
          </button>
        }
      />
    </div>
  );
}

function VaultRow({
  entry,
  keys,
  dragHandle,
}: {
  entry: VaultEntrySummary;
  keys: Keys;
  dragHandle?: ReactNode;
}) {
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
  const deleteMut = useDeleteEntry();
  const restoreMut = useRestoreEntry();

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

  function handleDelete() {
    if (!window.confirm(`Delete "${label}"? It can be restored within 30 days.`)) return;
    deleteMut.mutate(entry.id);
  }

  const deleted = Boolean(entry.deletedAt);
  const restorable = isRestorable(entry.deletedAt);
  const actionError = deleteMut.error ?? restoreMut.error;

  return (
    <div className="flex items-center justify-between gap-2 border-b py-2 last:border-b-0">
      <div className="flex min-w-0 items-center gap-2">
        {dragHandle}
        <div className="min-w-0">
          <p className="text-sm font-medium">{label}</p>
          <p className="text-xs text-muted-foreground">
            {entry.createdAt}
            {deleted && (
              <span className="ml-2 text-destructive">
                deleted {entry.deletedAt}
                {!restorable && " (restore window expired)"}
              </span>
            )}
          </p>
          {actionError && (
            <p className="mt-1 text-xs text-destructive">
              {actionError.message}
            </p>
          )}
        </div>
      </div>
      <div className="flex items-center gap-2">
        {deleted ? (
          restorable ? (
            <Button
              size="sm"
              variant="outline"
              disabled={restoreMut.isPending}
              onClick={() => restoreMut.mutate(entry.id)}
            >
              {restoreMut.isPending ? "..." : "Restore"}
            </Button>
          ) : null
        ) : (
          <>
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
            <Button
              size="sm"
              variant="destructive"
              disabled={deleteMut.isPending}
              onClick={handleDelete}
            >
              {deleteMut.isPending ? "..." : "Delete"}
            </Button>
          </>
        )}
      </div>
    </div>
  );
}

function CreateEntryButton({
  keys,
  nextSortOrder,
}: {
  keys: Keys;
  nextSortOrder: number;
}) {
  const [open, setOpen] = useState(false);
  const createMut = useCreateEntry();

  async function handleCreate(data: EntryFormData) {
    const { preview, bundle } = await encryptEntry(
      { label: data.label, files: data.files },
      [keys.recipient],
    );
    await createMut.mutateAsync({ preview, bundle, sortOrder: nextSortOrder });
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
