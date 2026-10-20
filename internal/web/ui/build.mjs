import { build } from "esbuild";
import { execFileSync } from "node:child_process";
import { copyFile, mkdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.dirname(fileURLToPath(import.meta.url));
const output = path.resolve(root, "../static");
await mkdir(output, { recursive: true });

execFileSync(path.join(root, "node_modules/.bin/tailwindcss"), [
	"-i", path.join(root, "input.css"),
	"-o", path.join(output, "app.css"),
	"--minify",
], { cwd: root, stdio: "inherit" });

await build({
	entryPoints: [path.join(root, "app.js")],
	bundle: true,
	format: "iife",
	minify: true,
	target: ["es2022"],
	outfile: path.join(output, "app.js"),
});
await Promise.all([
	copyFile(path.join(root, "index.html"), path.join(output, "index.html")),
	copyFile(path.join(root, "theme.css"), path.join(output, "theme.css")),
]);
