import { base64 } from "@scure/base";
import { sessionFetch } from "@/lib/session";

const BASE = "/api/v1/vault";

export type VaultEntrySummary = {
  id: string;
  preview: Uint8Array;
  sortOrder: number;
  schemaVersion: number;
  createdAt: string;
  updatedAt: string;
  deletedAt: string | null;
};

export type VaultEntry = VaultEntrySummary & {
  bundle: Uint8Array;
};

export type EntryWrite = {
  preview: Uint8Array;
  bundle: Uint8Array;
  sortOrder: number;
  recipientContactIds?: string[];
};

export type ListEntriesOptions = {
  includeDeleted?: boolean;
  limit?: number;
};

export type ListEntriesResponse = {
  entries: VaultEntrySummary[];
  nextCursor: string | null;
};

type WireSummary = {
  id: string;
  preview: string;
  sortOrder: number;
  schemaVersion: number;
  createdAt: string;
  updatedAt: string;
  deletedAt: string | null;
};

type WireEntry = WireSummary & {
  bundle: string;
};

type WireListResponse = {
  entries: WireSummary[];
  nextCursor: string | null;
};

type WireEntryWrite = {
  preview: string;
  bundle: string;
  sortOrder: number;
  recipientContactIds: string[];
};

function fromWireSummary(w: WireSummary): VaultEntrySummary {
  return {
    id: w.id,
    preview: base64.decode(w.preview),
    sortOrder: w.sortOrder,
    schemaVersion: w.schemaVersion,
    createdAt: w.createdAt,
    updatedAt: w.updatedAt,
    deletedAt: w.deletedAt,
  };
}

function fromWireEntry(w: WireEntry): VaultEntry {
  return {
    ...fromWireSummary(w),
    bundle: base64.decode(w.bundle),
  };
}

function toWireEntryWrite(w: EntryWrite): WireEntryWrite {
  return {
    preview: base64.encode(w.preview),
    bundle: base64.encode(w.bundle),
    sortOrder: w.sortOrder,
    recipientContactIds: w.recipientContactIds ?? [],
  };
}

export async function listEntries(
  options: ListEntriesOptions = {},
): Promise<ListEntriesResponse> {
  const params = new URLSearchParams();
  if (options.includeDeleted) params.set("includeDeleted", "true");
  if (options.limit !== undefined)
    params.set("limit", options.limit.toString());
  const query = params.toString();
  const url = query ? `${BASE}/entries?${query}` : `${BASE}/entries`;

  const res = await sessionFetch(url, { method: "GET" });
  if (!res.ok) {
    throw new Error(`listEntries failed: ${res.status} ${await res.text()}`);
  }
  const wire: WireListResponse = await res.json();
  return {
    entries: wire.entries.map(fromWireSummary),
    nextCursor: wire.nextCursor,
  };
}

export async function createEntry(body: EntryWrite): Promise<VaultEntry> {
  const res = await sessionFetch(`${BASE}/entries`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(toWireEntryWrite(body)),
  });
  if (!res.ok) {
    throw new Error(`createEntry failed: ${res.status} ${await res.text()}`);
  }
  return fromWireEntry(await res.json());
}
