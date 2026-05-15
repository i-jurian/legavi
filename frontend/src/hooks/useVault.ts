import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  createEntry,
  listEntries,
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
