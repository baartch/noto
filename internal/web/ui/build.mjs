import { copyFile, mkdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.dirname(fileURLToPath(import.meta.url));
const output = path.resolve(root, "../static");
await mkdir(output, { recursive: true });

const packageJSON = await Bun.file(path.join(root, "package.json")).json();
if (packageJSON.devDependencies?.esbuild || packageJSON.dependencies?.esbuild) {
	throw new Error("esbuild must not be a web UI build dependency; use Bun.build");
}

const tailwind = Bun.spawnSync([
	path.join(root, "node_modules/.bin/tailwindcss"),
	"-i", path.join(root, "input.css"),
	"-o", path.join(output, "app.css"),
	"--minify",
], { cwd: root, stdout: "inherit", stderr: "inherit" });
if (tailwind.exitCode !== 0) throw new Error(`Tailwind CLI failed with exit code ${tailwind.exitCode}`);

const bundle = await Bun.build({
	entrypoints: [path.join(root, "app.js")],
	outdir: output,
	format: "iife",
	minify: true,
	target: "browser",
	naming: "app.js",
});
if (!bundle.success) throw new AggregateError(bundle.logs, "Bun browser bundle failed");
await Promise.all([
	copyFile(path.join(root, "index.html"), path.join(output, "index.html")),
	copyFile(path.join(root, "theme.css"), path.join(output, "theme.css")),
]);
