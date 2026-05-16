import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createEntry,
  deleteEntry,
  listEntries,
  restoreEntry,
  updateEntry,
  type EntryWrite,
  type ListEntriesOptions,
} from "@/api/vault";

const entriesKey = ["vault", "entries"] as const;

export function useVaultEntries(options: ListEntriesOptions = {}) {
  return useQuery({
    queryKey: [...entriesKey, options],
    queryFn: () => listEntries(options),
  });
}

export function useCreateEntry() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: EntryWrite) => createEntry(body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: entriesKey });
    },
  });
}

export function useUpdateEntry() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: EntryWrite }) =>
      updateEntry(id, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: entriesKey });
    },
  });
}

export function useDeleteEntry() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => deleteEntry(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: entriesKey });
    },
  });
}

export function useRestoreEntry() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => restoreEntry(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: entriesKey });
    },
  });
}
