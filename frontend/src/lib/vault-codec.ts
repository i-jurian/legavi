import { Decrypter, Encrypter } from "age-encryption";
import {
  BlobReader,
  BlobWriter,
  Uint8ArrayWriter,
  ZipReader,
  ZipWriter,
} from "@zip.js/zip.js";

const VAULT_SCHEMA_VERSION = 1;

export type EntryInput = {
  label: string;
  files: File[];
};

export type Preview = {
  label: string;
  schemaVersion: number;
};

export type EncryptedEntry = {
  preview: Uint8Array;
  bundle: Uint8Array;
};

export async function encryptEntry(
  input: EntryInput,
  recipients: string[]
): Promise<EncryptedEntry> {
  if (recipients.length === 0) {
    throw new Error("encryptEntry requires at least one recipient");
  }

  const manifest: Preview = {
    label: input.label,
    schemaVersion: VAULT_SCHEMA_VERSION,
  };
  const manifestBytes = new TextEncoder().encode(JSON.stringify(manifest));
  const preview = await encryptToRecipients(manifestBytes, recipients);

  const zipBytes = await zipFiles(input.files);
  const bundle = await encryptToRecipients(zipBytes, recipients);

  return { preview, bundle };
}

export async function decryptPreview(
  preview: Uint8Array,
  identity: string
): Promise<Preview> {
  const text = await decryptText(preview, identity);
  const parsed: unknown = JSON.parse(text);
  if (!isPreview(parsed)) {
    throw new Error("preview manifest malformed");
  }
  return parsed;
}

export async function decryptBundle(
  bundle: Uint8Array,
  identity: string
): Promise<File[]> {
  const zipBytes = await decryptBytes(bundle, identity);
  const reader = new ZipReader(new BlobReader(new Blob([new Uint8Array(zipBytes)])));
  const entries = await reader.getEntries();
  const files: File[] = [];
  for (const entry of entries) {
    if (entry.directory || !entry.getData) continue;
    const blob = await entry.getData(new BlobWriter());
    files.push(new File([blob], entry.filename));
  }
  await reader.close();
  return files;
}

function isPreview(v: unknown): v is Preview {
  if (typeof v !== "object" || v === null) return false;
  const obj = v as Record<string, unknown>;
  return typeof obj.label === "string" && typeof obj.schemaVersion === "number";
}

async function encryptToRecipients(
  plaintext: Uint8Array,
  recipients: string[]
): Promise<Uint8Array> {
  const enc = new Encrypter();
  for (const r of recipients) {
    enc.addRecipient(r);
  }
  return enc.encrypt(plaintext);
}

async function decryptText(
  ciphertext: Uint8Array,
  identity: string
): Promise<string> {
  const dec = new Decrypter();
  dec.addIdentity(identity);
  return dec.decrypt(ciphertext, "text");
}

async function decryptBytes(
  ciphertext: Uint8Array,
  identity: string
): Promise<Uint8Array> {
  const dec = new Decrypter();
  dec.addIdentity(identity);
  return dec.decrypt(ciphertext, "uint8array");
}

async function zipFiles(files: File[]): Promise<Uint8Array> {
  const writer = new ZipWriter(new Uint8ArrayWriter());
  for (const f of files) {
    await writer.add(f.name, new BlobReader(f));
  }
  return writer.close();
}
