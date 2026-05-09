import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import sri from "./plugins/vite-plugin-sri";

export default defineConfig({
  plugins: [react(), tailwindcss(), sri()],
});
