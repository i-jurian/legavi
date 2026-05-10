/**
 * Vendored from vite-plugin-sri3 (MIT).
 * See plugins/vite-plugin-sri.LICENSE for the original copyright notice.
 */

import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import path from "node:path";
import type { Plugin } from "vite";

const VITE_INTERNAL_ANALYSIS_PLUGIN = "vite:build-import-analysis";
const EXTERNAL_SCRIPT_RE =
  /<script[^<>]*['"]*src['"]*=['"]*([^ '"]+)['"]*[^<>]*><\/script>/g;
const EXTERNAL_CSS_RE =
  /<link[^<>]*['"]*rel['"]*=['"]*stylesheet['"]*[^<>]+['"]*href['"]*=['"]([^^ '"]+)['"][^<>]*>/g;
const EXTERNAL_MODULE_RE =
  /<link[^<>]*['"]*rel['"]*=['"]*modulepreload['"]*[^<>]+['"]*href['"]*=['"]([^^ '"]+)['"][^<>]*>/g;
const SKIP_SRI_ATTR_RE =
  /\s+skip-sri(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>/]+))?/i;

export interface SriOptions {
  ignoreMissingAsset?: boolean;
}

interface RangeChange {
  start: number;
  end: number;
  content: string;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type AnyHook = (...args: any[]) => unknown;

function hijackGenerateBundle(plugin: Plugin, afterHook: AnyHook): void {
  const hook = plugin.generateBundle;
  if (typeof hook === "object" && hook && "handler" in hook && hook.handler) {
    const fn = hook.handler as AnyHook;
    (hook as { handler: AnyHook }).handler = async function (
      this: unknown,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      ...args: any[]
    ) {
      await fn.apply(this, args);
      await afterHook?.apply(this, args);
    };
    return;
  }
  if (typeof hook === "function") {
    const fn = hook as AnyHook;
    plugin.generateBundle = async function (
      this: unknown,
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      ...args: any[]
    ) {
      await fn.apply(this, args);
      await afterHook?.apply(this, args);
    } as Plugin["generateBundle"];
  }
}

export function sri(options?: SriOptions): Plugin {
  const { ignoreMissingAsset = false } = options ?? {};
  return {
    name: "vite-plugin-sri3",
    enforce: "post",
    apply: "build",
    configResolved(config) {
      const generateBundle = async function (
        _: unknown,
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        bundle: Record<string, any>,
      ): Promise<void> {
        const isRemoteUrl = (url: string): string | false => {
          if (url.startsWith("//")) return `https:${url}`;
          if (/^https?:\/\//i.test(url)) return url;
          return false;
        };

        const normalizeBaseUrl = (url: string): string => {
          if (config.base === "./" || config.base === "") return url;
          const base = config.base.endsWith("/")
            ? config.base
            : `${config.base}/`;
          return url.startsWith(base) ? url.slice(base.length) : url;
        };

        const getBundleKey = (htmlPath: string, url: string): string => {
          if (config.base === "./" || config.base === "") {
            return path.posix.normalize(
              path.posix.join(path.posix.dirname(htmlPath), url),
            );
          }
          return normalizeBaseUrl(url);
        };

        const readPublicAsset = async (
          htmlPath: string,
          url: string,
        ): Promise<Buffer | null> => {
          const publicDir = config.publicDir;
          if (!publicDir) return null;
          let publicUrl = normalizeBaseUrl(url);
          publicUrl = publicUrl.split(/[?#]/)[0];
          if (config.base === "./" || config.base === "") {
            publicUrl = path.posix.normalize(
              path.posix.join(path.posix.dirname(htmlPath), publicUrl),
            );
          }
          let decoded: string;
          try {
            decoded = decodeURIComponent(publicUrl);
          } catch {
            decoded = publicUrl;
          }
          const filePath = path.resolve(publicDir, decoded);
          const rel = path.relative(publicDir, filePath);
          if (rel.startsWith("..") || path.isAbsolute(rel)) return null;
          try {
            return await readFile(filePath);
          } catch {
            return null;
          }
        };

        const calculateIntegrity = async (
          source: string | Uint8Array,
        ): Promise<string> =>
          `sha384-${createHash("sha384").update(source).digest().toString("base64")}`;

        const getAssetSource = async (
          htmlPath: string,
          url: string,
        ): Promise<string | Uint8Array | null> => {
          const remoteUrl = isRemoteUrl(url);
          if (remoteUrl) {
            return new Uint8Array(await (await fetch(remoteUrl)).arrayBuffer());
          }
          const bundleItem = bundle[getBundleKey(htmlPath, url)];
          if (bundleItem) {
            return bundleItem.type === "chunk"
              ? bundleItem.code
              : bundleItem.source;
          }
          const publicAsset = await readPublicAsset(htmlPath, url);
          if (!publicAsset) {
            if (ignoreMissingAsset) return null;
            throw new Error(
              `Asset ${url} not found in bundle/public directory.`,
            );
          }
          return publicAsset;
        };

        const transformHTML = async (
          regex: RegExp,
          endOffset: number,
          htmlPath: string,
          html: string,
        ): Promise<string> => {
          let match: RegExpExecArray | null;
          const changes: RangeChange[] = [];
          while ((match = regex.exec(html))) {
            const [rawMatch, url] = match;
            const start = match.index;
            const end = regex.lastIndex;
            const skipMatch = SKIP_SRI_ATTR_RE.exec(rawMatch);
            if (skipMatch) {
              changes.push({
                start: start + skipMatch.index,
                end: start + skipMatch.index + skipMatch[0].length,
                content: "",
              });
              continue;
            }
            const source = await getAssetSource(htmlPath, url);
            if (!source) continue;
            const integrity = await calculateIntegrity(source);
            const insertPos = end - endOffset;
            changes.push({
              start: insertPos,
              end: insertPos,
              content: ` integrity="${integrity}"`,
            });
          }
          for (let i = changes.length - 1; i >= 0; i--) {
            const { start, end, content } = changes[i];
            html = html.slice(0, start) + content + html.slice(end);
          }
          return html;
        };

        for (const name in bundle) {
          const chunk = bundle[name];
          if (
            chunk.type === "asset" &&
            (chunk.fileName.endsWith(".html") ||
              chunk.fileName.endsWith(".htm"))
          ) {
            let html = chunk.source.toString();
            html = await transformHTML(EXTERNAL_SCRIPT_RE, 10, name, html);
            html = await transformHTML(EXTERNAL_CSS_RE, 1, name, html);
            html = await transformHTML(EXTERNAL_MODULE_RE, 1, name, html);
            chunk.source = html;
          }
        }
      };

      const plugin = config.plugins.find(
        (p) => p.name === VITE_INTERNAL_ANALYSIS_PLUGIN,
      );
      if (!plugin) {
        throw new Error(
          "vite-plugin-sri cannot work in versions lower than Vite 2.0.0",
        );
      }
      hijackGenerateBundle(plugin, generateBundle);
    },
  };
}

export default sri;
