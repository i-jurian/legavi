import { describe, expect, it } from "vitest";
import { bech32 } from "@scure/base";
import { generateX25519Identity, identityToRecipient } from "age-encryption";
import { deriveAgeKeypair } from "./age-keypair";

describe("deriveAgeKeypair", () => {
  const sampleInput = new Uint8Array(32).fill(0x42);

  it("rejects non-32-byte input", () => {
    expect(() => deriveAgeKeypair(new Uint8Array(31))).toThrow();
    expect(() => deriveAgeKeypair(new Uint8Array(33))).toThrow();
  });

  it("is deterministic", () => {
    const a = deriveAgeKeypair(sampleInput);
    const b = deriveAgeKeypair(sampleInput);
    expect(a).toEqual(b);
  });

  it("produces age-format strings", () => {
    const { identity: identity, recipient: recipient } =
      deriveAgeKeypair(sampleInput);
    expect(identity.startsWith("AGE-SECRET-KEY-1")).toBe(true);
    expect(recipient.startsWith("age1")).toBe(true);
  });

  it("different inputs produce different outputs", () => {
    const a = deriveAgeKeypair(new Uint8Array(32).fill(0x01));
    const b = deriveAgeKeypair(new Uint8Array(32).fill(0x02));
    expect(a.identity).not.toBe(b.identity);
    expect(a.recipient).not.toBe(b.recipient);
  });

  it("matches age-encryption's reference implementation", async () => {
    const libIdentity = await generateX25519Identity();
    const libRecipient = await identityToRecipient(libIdentity);

    const { bytes: privateBytes } = bech32.decodeToBytes(libIdentity);

    const ours = deriveAgeKeypair(privateBytes);

    expect(ours.identity).toBe(libIdentity);
    expect(ours.recipient).toBe(libRecipient);
  });
});
